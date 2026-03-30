package commands

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

// ParsedURL holds the components extracted from a DeployHQ URL.
type ParsedURL struct {
	Account    string `json:"account"`
	Resource   string `json:"resource"`
	Project    string `json:"project,omitempty"`
	SubResource string `json:"sub_resource,omitempty"`
	ID         string `json:"id,omitempty"`
	RawURL     string `json:"raw_url"`
}

func newURLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "url",
		Short: "URL tools",
	}

	cmd.AddCommand(newURLParseCmd())
	return cmd
}

func newURLParseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "parse <url>",
		Short: "Parse a DeployHQ URL into components",
		Long: `Parse a DeployHQ URL and extract project, resource, and identifier.

Examples:
  deployhq url parse https://myco.deployhq.com/projects/my-app
  deployhq url parse https://myco.deployhq.com/projects/my-app/deployments/abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parsed, err := parseDeployHQURL(args[0])
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(parsed, fmt.Sprintf("Parsed: %s", parsed.Resource)))
			}

			rows := [][]string{
				{"Account", parsed.Account},
				{"Resource", parsed.Resource},
			}
			if parsed.Project != "" {
				rows = append(rows, []string{"Project", parsed.Project})
			}
			if parsed.SubResource != "" {
				rows = append(rows, []string{"Sub-resource", parsed.SubResource})
			}
			if parsed.ID != "" {
				rows = append(rows, []string{"ID", parsed.ID})
			}
			env.WriteTable([]string{"Field", "Value"}, rows)
			return nil
		},
	}
}

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <url>",
		Short: "Show any DeployHQ resource by URL",
		Long: `Fetch and display any DeployHQ resource given its URL.

Examples:
  deployhq show https://myco.deployhq.com/projects/my-app
  deployhq show https://myco.deployhq.com/projects/my-app/deployments/abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parsed, err := parseDeployHQURL(args[0])
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			ctx := cliCtx.Background()

			switch parsed.Resource {
			case "projects":
				if parsed.SubResource != "" && parsed.Project != "" {
					// Sub-resource: /projects/<project>/<sub>/<id>
					return showSubResource(parsed)
				}
				if parsed.Project != "" {
					project, err := client.GetProject(ctx, parsed.Project)
					if err != nil {
						return err
					}
					return env.WriteJSON(output.NewResponse(project, fmt.Sprintf("Project: %s", project.Name)))
				}
				projects, err := client.ListProjects(ctx)
				if err != nil {
					return err
				}
				return env.WriteJSON(output.NewResponse(projects, fmt.Sprintf("%d projects", len(projects))))

			default:
				// Fall back to raw API call
				var result interface{}
				path := buildAPIPath(parsed)
				if err := client.Do(ctx, "GET", path, nil, &result); err != nil {
					return err
				}
				return env.WriteJSON(result)
			}
		},
	}
}

func showSubResource(parsed *ParsedURL) error {
	client, err := cliCtx.Client()
	if err != nil {
		return err
	}

	ctx := cliCtx.Background()
	env := cliCtx.Envelope

	switch parsed.SubResource {
	case "deployments":
		if parsed.ID != "" {
			dep, err := client.GetDeployment(ctx, parsed.Project, parsed.ID)
			if err != nil {
				return err
			}
			return env.WriteJSON(output.NewResponse(dep, fmt.Sprintf("Deployment: %s", dep.Identifier)))
		}
		deps, err := client.ListDeployments(ctx, parsed.Project)
		if err != nil {
			return err
		}
		return env.WriteJSON(output.NewResponse(deps, fmt.Sprintf("%d deployments", len(deps.Records))))

	case "servers":
		if parsed.ID != "" {
			srv, err := client.GetServer(ctx, parsed.Project, parsed.ID)
			if err != nil {
				return err
			}
			return env.WriteJSON(output.NewResponse(srv, fmt.Sprintf("Server: %s", srv.Name)))
		}
		servers, err := client.ListServers(ctx, parsed.Project)
		if err != nil {
			return err
		}
		return env.WriteJSON(output.NewResponse(servers, fmt.Sprintf("%d servers", len(servers))))

	default:
		// Generic API call
		var result interface{}
		path := buildAPIPath(parsed)
		if err := client.Do(ctx, "GET", path, nil, &result); err != nil {
			return err
		}
		return env.WriteJSON(result)
	}
}

func parseDeployHQURL(rawURL string) (*ParsedURL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, &output.UserError{
			Message: fmt.Sprintf("Invalid URL: %s", rawURL),
			Hint:    "Provide a valid DeployHQ URL like https://account.deployhq.com/projects/my-app",
		}
	}

	host := u.Hostname()
	if !strings.HasSuffix(host, ".deployhq.com") {
		return nil, &output.UserError{
			Message: fmt.Sprintf("Not a DeployHQ URL: %s", host),
			Hint:    "URL must be *.deployhq.com",
		}
	}

	account := strings.TrimSuffix(host, ".deployhq.com")
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")

	parsed := &ParsedURL{
		Account: account,
		RawURL:  rawURL,
	}

	if len(parts) >= 1 {
		parsed.Resource = parts[0]
	}
	if len(parts) >= 2 {
		parsed.Project = parts[1]
	}
	if len(parts) >= 3 {
		parsed.SubResource = parts[2]
	}
	if len(parts) >= 4 {
		parsed.ID = parts[3]
	}

	return parsed, nil
}

func buildAPIPath(parsed *ParsedURL) string {
	path := "/" + parsed.Resource
	if parsed.Project != "" {
		path += "/" + parsed.Project
	}
	if parsed.SubResource != "" {
		path += "/" + parsed.SubResource
	}
	if parsed.ID != "" {
		path += "/" + parsed.ID
	}
	return path
}
