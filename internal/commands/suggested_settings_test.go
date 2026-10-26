package commands

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/detect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProjectSettings is a stateful fake for a project's build commands,
// excluded files and build cache files, so apply → re-plan can be tested.
type fakeProjectSettings struct {
	mu            sync.Mutex
	buildCommands []string
	excluded      []string
	cache         []string
	posts         int
	excludedPosts []string // raw request bodies of excluded-file creates
}

func (f *fakeProjectSettings) server(t *testing.T) *httptest.Server {
	t.Helper()
	list := func(w http.ResponseWriter, items []string, cmd bool) {
		var parts []string
		for i, it := range items {
			if cmd {
				parts = append(parts, `{"identifier":"bc-`+string(rune('a'+i))+`","command":"`+it+`"}`)
			} else {
				parts = append(parts, `{"identifier":"x-`+string(rune('a'+i))+`","path":"`+it+`"}`)
			}
		}
		_, _ = w.Write([]byte("[" + strings.Join(parts, ",") + "]"))
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		field := func(name string) string {
			s := string(body)
			i := strings.Index(s, `"`+name+`":"`)
			if i < 0 {
				return ""
			}
			s = s[i+len(name)+4:]
			return s[:strings.Index(s, `"`)]
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/build_commands") && r.Method == http.MethodGet:
			list(w, f.buildCommands, true)
		case strings.HasSuffix(r.URL.Path, "/build_commands") && r.Method == http.MethodPost:
			f.posts++
			f.buildCommands = append(f.buildCommands, field("command"))
			_, _ = w.Write([]byte(`{"identifier":"bc-new"}`))
		case strings.HasSuffix(r.URL.Path, "/excluded_files") && r.Method == http.MethodGet:
			list(w, f.excluded, false)
		case strings.HasSuffix(r.URL.Path, "/excluded_files") && r.Method == http.MethodPost:
			f.posts++
			f.excludedPosts = append(f.excludedPosts, string(body))
			f.excluded = append(f.excluded, field("path"))
			_, _ = w.Write([]byte(`{"identifier":"x-new","path":"` + field("path") + `"}`))
		case strings.HasSuffix(r.URL.Path, "/build_cache_files") && r.Method == http.MethodGet:
			list(w, f.cache, false)
		case strings.HasSuffix(r.URL.Path, "/build_cache_files") && r.Method == http.MethodPost:
			f.posts++
			f.cache = append(f.cache, field("path"))
			_, _ = w.Write([]byte(`{"identifier":"c-new","path":"` + field("path") + `"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

var nodeDetection = detect.Result{
	Framework: "nextjs",
	BuildCommands: []detect.BuildCommandStep{
		{Description: "Install dependencies", Command: "npm ci"},
		{Description: "Build", Command: "npm run build"},
	},
	ExcludedFiles:   []string{"node_modules/", ".env"},
	BuildCacheFiles: []string{"node_modules/"},
}

func statuses(items []suggestedItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Status)
	}
	return out
}

// Planning makes no changes and diffs against what the project already has.
func TestPlanSuggestedSettings_DiffsAgainstProject(t *testing.T) {
	f := &fakeProjectSettings{excluded: []string{".env"}}
	client := newTestClient(t, f.server(t))

	plan := planSuggestedSettings(t.Context(), client, "my-app", nodeDetection)

	assert.Equal(t, []string{"add", "add"}, statuses(plan.BuildCommands))
	assert.Equal(t, []string{"add", "exists"}, statuses(plan.ExcludedFiles))
	assert.Equal(t, []string{"add"}, statuses(plan.BuildCacheFiles))
	assert.Equal(t, 4, plan.pending())
	assert.Zero(t, f.posts, "planning must not create anything")
}

// An existing build pipeline is never extended or duplicated.
func TestPlanSuggestedSettings_SkipsBuildCommandsWhenProjectHasSome(t *testing.T) {
	f := &fakeProjectSettings{buildCommands: []string{"make"}}
	plan := planSuggestedSettings(t.Context(), newTestClient(t, f.server(t)), "my-app", nodeDetection)
	assert.Equal(t, []string{"skipped", "skipped"}, statuses(plan.BuildCommands))
}

// Applying creates only pending items, and a re-run has nothing left to do.
func TestApplySuggestedSettings_IdempotentOnRerun(t *testing.T) {
	f := &fakeProjectSettings{excluded: []string{".env"}}
	client := newTestClient(t, f.server(t))
	env, _, _ := testLaunchEnvelope()

	plan := planSuggestedSettings(t.Context(), client, "my-app", nodeDetection)
	applySuggestedSettings(t.Context(), env, client, plan)

	assert.Equal(t, []string{"added", "added"}, statuses(plan.BuildCommands))
	assert.Equal(t, []string{"added", "exists"}, statuses(plan.ExcludedFiles))
	assert.Equal(t, []string{"added"}, statuses(plan.BuildCacheFiles))
	assert.Equal(t, []string{"npm ci", "npm run build"}, f.buildCommands, "steps are created in order")
	assert.Equal(t, 4, f.posts)

	again := planSuggestedSettings(t.Context(), client, "my-app", nodeDetection)
	assert.Zero(t, again.pending(), "second run must find nothing to apply")
	applySuggestedSettings(t.Context(), env, client, again)
	assert.Equal(t, 4, f.posts, "second run must not create anything")
}

// Suggested exclusions apply to every server. Without all_servers the API
// attaches the rule to no server, so the file would still be uploaded.
func TestApplySuggestedSettings_ExcludedFilesApplyToAllServers(t *testing.T) {
	f := &fakeProjectSettings{}
	client := newTestClient(t, f.server(t))
	env, _, _ := testLaunchEnvelope()

	applySuggestedSettings(t.Context(), env, client, planSuggestedSettings(t.Context(), client, "my-app", nodeDetection))

	require.Len(t, f.excludedPosts, 2)
	for _, body := range f.excludedPosts {
		assert.Contains(t, body, `"all_servers":true`)
	}
}

// The plan summary names every item and its status on stderr.
func TestPrintSuggestedSettingsPlan(t *testing.T) {
	f := &fakeProjectSettings{buildCommands: []string{"make"}}
	plan := planSuggestedSettings(t.Context(), newTestClient(t, f.server(t)), "my-app", nodeDetection)
	env, stdout, stderr := testLaunchEnvelope()

	printSuggestedSettingsPlan(env, plan)
	out := stderr.String()
	assert.Contains(t, out, "[skipped] npm ci")
	assert.Contains(t, out, "[add] node_modules/")
	assert.Contains(t, out, "already has some")
	assert.Empty(t, stdout.String(), "human summary goes to stderr only")
}

func TestProjectsApplySuggestions_Registered(t *testing.T) {
	cmd := NewRootCmd("test")
	sub, _, err := cmd.Find([]string{"projects", "apply-suggestions"})
	require.NoError(t, err)
	assert.Equal(t, "apply-suggestions", sub.Name())
	assert.NotNil(t, sub.Flags().Lookup("dry-run"))
}
