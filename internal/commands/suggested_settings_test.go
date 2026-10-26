package commands

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/detect"
	"github.com/deployhq/deployhq-cli/internal/output"
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

	plan, err := planSuggestedSettings(t.Context(), client, "my-app", nodeDetection)
	require.NoError(t, err)

	assert.Equal(t, []string{"add", "add"}, statuses(plan.BuildCommands))
	assert.Equal(t, []string{"add", "exists"}, statuses(plan.ExcludedFiles))
	assert.Equal(t, []string{"add"}, statuses(plan.BuildCacheFiles))
	assert.Equal(t, 4, plan.pending())
	assert.Zero(t, f.posts, "planning must not create anything")
}

// An existing build pipeline is never extended or duplicated.
func TestPlanSuggestedSettings_SkipsBuildCommandsWhenProjectHasSome(t *testing.T) {
	f := &fakeProjectSettings{buildCommands: []string{"make"}}
	plan, err := planSuggestedSettings(t.Context(), newTestClient(t, f.server(t)), "my-app", nodeDetection)
	require.NoError(t, err)
	assert.Equal(t, []string{"skipped", "skipped"}, statuses(plan.BuildCommands))
}

// Applying creates only pending items, and a re-run has nothing left to do.
func TestApplySuggestedSettings_IdempotentOnRerun(t *testing.T) {
	f := &fakeProjectSettings{excluded: []string{".env"}}
	client := newTestClient(t, f.server(t))
	env, _, _ := testLaunchEnvelope()

	plan, err := planSuggestedSettings(t.Context(), client, "my-app", nodeDetection)
	require.NoError(t, err)
	applySuggestedSettings(t.Context(), env, client, plan)

	assert.Equal(t, []string{"added", "added"}, statuses(plan.BuildCommands))
	assert.Equal(t, []string{"added", "exists"}, statuses(plan.ExcludedFiles))
	assert.Equal(t, []string{"added"}, statuses(plan.BuildCacheFiles))
	assert.Equal(t, []string{"npm ci", "npm run build"}, f.buildCommands, "steps are created in order")
	assert.Equal(t, 4, f.posts)

	again, err := planSuggestedSettings(t.Context(), client, "my-app", nodeDetection)
	require.NoError(t, err)
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

	plan, err := planSuggestedSettings(t.Context(), client, "my-app", nodeDetection)
	require.NoError(t, err)
	applySuggestedSettings(t.Context(), env, client, plan)

	require.Len(t, f.excludedPosts, 2)
	for _, body := range f.excludedPosts {
		assert.Contains(t, body, `"all_servers":true`)
	}
}

// The plan summary names every item and its status on stderr.
func TestPrintSuggestedSettingsPlan(t *testing.T) {
	f := &fakeProjectSettings{buildCommands: []string{"make"}}
	plan, err := planSuggestedSettings(t.Context(), newTestClient(t, f.server(t)), "my-app", nodeDetection)
	require.NoError(t, err)
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

// fakeSuggestionsAPI answers every request 'dhq projects apply-suggestions'
// makes for project my-app, through http.DefaultTransport (the SDK client the
// real command builds has no other seam). Settings lists are empty; status
// overrides a list or create response for a settings path suffix
// ("/build_commands", "/excluded_files", "/build_cache_files"), keyed by
// "GET <suffix>" or "POST <suffix>".
type fakeSuggestionsAPI struct {
	mu     sync.Mutex
	status map[string]int
	posts  map[string]int // successful or not, by settings path suffix
}

func (f *fakeSuggestionsAPI) install(t *testing.T) {
	t.Helper()
	f.posts = map[string]int{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		path := r.URL.Path
		switch {
		case r.Method == http.MethodGet && path == "/projects/my-app":
			_, _ = w.Write([]byte(`{"name":"My App","permalink":"my-app","identifier":"p-1"}`))
			return
		case r.Method == http.MethodPost && path == "/detection":
			_, _ = w.Write([]byte(`{"stack":"nextjs",` +
				`"build_commands":[{"description":"Install","command":"npm ci"}],` +
				`"excluded_files":[{"path":"node_modules/"}],` +
				`"build_cache_files":[{"path":".next/cache/"}]}`))
			return
		}
		for _, suffix := range []string{"/build_commands", "/excluded_files", "/build_cache_files"} {
			if !strings.HasSuffix(path, "/projects/my-app"+suffix) {
				continue
			}
			if r.Method == http.MethodPost {
				f.posts[suffix]++
			}
			if code := f.status[r.Method+" "+suffix]; code != 0 {
				w.WriteHeader(code)
				_, _ = w.Write([]byte(`{"error":"failed"}`))
				return
			}
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`[]`))
			} else {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"identifier":"new"}`))
			}
			return
		}
		// Anything else (e.g. the root command's update check) is not part of
		// what these tests assert.
		_, _ = w.Write([]byte(`{}`))
	})

	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Type", "application/json")
		handler.ServeHTTP(rec, r)
		resp := rec.Result()
		resp.Request = r
		return resp, nil
	})
	t.Cleanup(func() { http.DefaultTransport = orig })
}

// runApplySuggestions runs the real 'dhq projects apply-suggestions --json' in
// an empty directory and returns the JSON document it wrote and its error.
func runApplySuggestions(t *testing.T) (string, error) {
	t.Helper()
	withResolvableContext(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	jsonl := filepath.Join(t.TempDir(), "out.jsonl")
	t.Setenv("DEPLOYHQ_OUTPUT_FILE", jsonl)

	err := runServersCmd(t, "projects", "apply-suggestions", "--json")
	written, _ := os.ReadFile(jsonl)
	return string(written), err
}

// A partial failure must not look like success: the command exits non-zero,
// names what failed, and the JSON result still carries the per-item outcome.
func TestProjectsApplySuggestions_FailsWhenAnItemFails(t *testing.T) {
	api := &fakeSuggestionsAPI{status: map[string]int{"POST /excluded_files": http.StatusUnprocessableEntity}}
	api.install(t)

	written, err := runApplySuggestions(t)

	require.Error(t, err, "a failed item must fail the command")
	assert.Equal(t, output.ExitUserError, output.ClassifyError(err))
	assert.Contains(t, err.Error(), "node_modules/", "the failed item is named")
	assert.NotContains(t, err.Error(), "npm ci", "items that were added are not reported as failed")
	assert.Equal(t, 1, api.posts["/build_commands"], "the other items are still applied")
	assert.Equal(t, 1, api.posts["/build_cache_files"], "the other items are still applied")

	var resp struct {
		OK   bool                  `json:"ok"`
		Data suggestedSettingsPlan `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(strings.SplitN(written, "\n", 2)[0]), &resp), "JSON result: %q", written)
	assert.False(t, resp.OK)
	require.Len(t, resp.Data.ExcludedFiles, 1)
	assert.Equal(t, suggestionFailed, resp.Data.ExcludedFiles[0].Status)
	assert.Equal(t, []string{suggestionAdded}, statuses(resp.Data.BuildCommands))
}

// Control for the failure tests: with every request succeeding, the same
// command run exits zero and adds every item.
func TestProjectsApplySuggestions_SucceedsWhenEveryItemIsAdded(t *testing.T) {
	api := &fakeSuggestionsAPI{}
	api.install(t)

	written, err := runApplySuggestions(t)

	require.NoError(t, err)
	assert.Equal(t, map[string]int{"/build_commands": 1, "/excluded_files": 1, "/build_cache_files": 1}, api.posts)
	assert.Contains(t, written, `"ok":true`)
}

// Build commands are only added to a project that has none. If the existing
// ones can't be listed, that can't be known, so the command stops rather than
// adding a second pipeline on top of one that may exist.
func TestProjectsApplySuggestions_StopsWhenBuildCommandsCannotBeListed(t *testing.T) {
	api := &fakeSuggestionsAPI{status: map[string]int{"GET /build_commands": http.StatusInternalServerError}}
	api.install(t)

	_, err := runApplySuggestions(t)

	require.Error(t, err, "an unknown build pipeline must stop the command")
	assert.Zero(t, api.posts["/build_commands"], "no build command may be added")
}

// Excluded files and build cache files are only added when missing. If the
// existing ones can't be listed, every suggestion would be posted again, so
// the command stops before changing anything.
func TestProjectsApplySuggestions_StopsWhenExistingPathsCannotBeListed(t *testing.T) {
	for _, suffix := range []string{"/excluded_files", "/build_cache_files"} {
		t.Run(strings.TrimPrefix(suffix, "/"), func(t *testing.T) {
			api := &fakeSuggestionsAPI{status: map[string]int{"GET " + suffix: http.StatusInternalServerError}}
			api.install(t)

			_, err := runApplySuggestions(t)

			require.Error(t, err, "unknown existing paths must stop the command")
			assert.Empty(t, api.posts, "nothing may be added")
		})
	}
}
