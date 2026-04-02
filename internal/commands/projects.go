package commands

import (
	"fmt"
	"os"
	"sort"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newProjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "projects",
		Aliases: []string{"project", "proj"},
		Short:   "Manage projects",
	}

	cmd.AddCommand(
		newProjectsListCmd(),
		newProjectsShowCmd(),
		newProjectsCreateCmd(),
		newProjectsUpdateCmd(),
		newProjectsDeleteCmd(),
		newProjectsStarCmd(),
		newProjectsInsightsCmd(),
		newProjectsUploadKeyCmd(),
		newProjectsBadgeCmd(),
	)

	return cmd
}

func newProjectsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects (starred first)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			projects, err := client.ListProjects(cliCtx.Background())
			if err != nil {
				return err
			}

			// Sort: starred projects first, then alphabetically by name within each group
			sort.SliceStable(projects, func(i, j int) bool {
				if projects[i].Starred != projects[j].Starred {
					return projects[i].Starred
				}
				return projects[i].Name < projects[j].Name
			})

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(projects,
					fmt.Sprintf("%d projects", len(projects)),
					output.Breadcrumb{Action: "show", Cmd: "dhq projects show <permalink>"},
					output.Breadcrumb{Action: "create", Cmd: "dhq projects create --name <name>"},
					output.Breadcrumb{Action: "star", Cmd: "dhq projects star <permalink>"},
				))
			}

			columns := []string{"*", "Name", "Permalink", "Zone", "Last Deployed"}
			rows := make([][]string, len(projects))
			for i, p := range projects {
				star := " "
				if p.Starred {
					star = "*"
				}
				lastDeploy := "-"
				if p.LastDeployedAt != nil {
					lastDeploy = *p.LastDeployedAt
				}
				rows[i] = []string{star, p.Name, p.Permalink, p.Zone, lastDeploy}
			}
			env.WriteTable(columns, rows)

			if len(projects) > 0 {
				env.Status("\nTip: dhq projects show %s", projects[0].Permalink)
				env.Status("     dhq projects star <permalink>  (toggle favourite)")
			}
			return nil
		},
	}
}

func newProjectsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "show [permalink]",
		Short:             "Show project details",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeProjectNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := resolveProjectArg(args)
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			project, err := client.GetProject(cliCtx.Background(), projectID)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(project,
					fmt.Sprintf("Project: %s", project.Name),
					output.Breadcrumb{Action: "servers", Cmd: fmt.Sprintf("dhq servers list -p %s", project.Permalink)},
					output.Breadcrumb{Action: "deployments", Cmd: fmt.Sprintf("dhq deployments list -p %s", project.Permalink)},
					output.Breadcrumb{Action: "deploy", Cmd: fmt.Sprintf("dhq deploy -p %s", project.Permalink)},
				))
			}

			env.WriteTable([]string{"Field", "Value"}, [][]string{
				{"Name", project.Name},
				{"Permalink", project.Permalink},
				{"Identifier", project.Identifier},
				{"Zone", project.Zone},
				{"Auto Deploy URL", project.AutoDeployURL},
			})

			servers, err := client.ListServers(cliCtx.Background(), project.Permalink)
			if err != nil {
				return nil // non-fatal: just skip server listing
			}
			if len(servers) > 0 {
				env.Status("\nServers:")
				srvCols := []string{"Name", "Identifier", "Protocol", "Branch"}
				srvRows := make([][]string, len(servers))
				for i, s := range servers {
					srvRows[i] = []string{s.Name, s.Identifier, s.ProtocolType, s.PreferredBranch}
				}
				env.WriteTable(srvCols, srvRows)
			}

			p := project.Permalink
			env.Status("\nNext commands:")
			env.Status("  dhq deployments list -p %s", p)
			env.Status("  dhq servers list -p %s", p)
			env.Status("  dhq env-vars list -p %s", p)
			env.Status("  dhq config-files list -p %s", p)
			env.Status("  dhq excluded-files list -p %s", p)
			env.Status("  dhq integrations list -p %s", p)
			env.Status("  dhq ssh-commands list -p %s", p)
			env.Status("  dhq auto-deploys list -p %s", p)
			env.Status("  dhq build-commands list -p %s", p)
			env.Status("  dhq build-configs list -p %s", p)
			env.Status("  dhq repos show -p %s", p)
			env.Status("  dhq deploy -p %s", p)
			env.Status("  dhq open %s", p)
			return nil
		},
	}
}

func newProjectsCreateCmd() *cobra.Command {
	var name, zone, template string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return &output.UserError{Message: "Project name is required", Hint: "Use --name flag"}
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			req := sdk.ProjectCreateRequest{Name: name, ZoneID: zone, TemplateID: template}
			project, err := client.CreateProject(cliCtx.Background(), req)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(project,
					fmt.Sprintf("Created project: %s", project.Name),
					output.Breadcrumb{Action: "servers", Cmd: fmt.Sprintf("dhq servers create -p %s --name <name> --protocol-type ssh", project.Permalink)},
				))
			}

			env.Status("Created project: %s (%s)", project.Name, project.Permalink)
			env.Status("\nNext: dhq servers create -p %s --name <name> --protocol-type ssh", project.Permalink)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Project name (required)")
	cmd.Flags().StringVar(&zone, "zone", "", "Zone ID")
	cmd.Flags().StringVar(&template, "template", "", "Template ID")
	return cmd
}

func newProjectsUpdateCmd() *cobra.Command {
	var (
		name                   string
		permalink              string
		zone                   string
		emailNotifyOn          string
		notificationEmail      string
		notifyPusher           bool
		checkUndeployedChanges string
		storeArtifactsEnabled  string
	)

	cmd := &cobra.Command{
		Use:   "update [permalink]",
		Short: "Update a project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := resolveProjectArg(args)
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			req := sdk.ProjectUpdateRequest{}
			if cmd.Flags().Changed("name") {
				req.Name = name
			}
			if cmd.Flags().Changed("permalink") {
				req.Permalink = permalink
			}
			if cmd.Flags().Changed("zone") {
				req.ZoneID = zone
			}
			if cmd.Flags().Changed("email-notify-on") {
				req.EmailNotifyOn = emailNotifyOn
			}
			if cmd.Flags().Changed("notification-email") {
				req.NotificationEmail = notificationEmail
			}
			if cmd.Flags().Changed("notify-pusher") {
				req.NotifyPusher = &notifyPusher
			}
			if cmd.Flags().Changed("check-undeployed-changes") {
				req.CheckUndeployedChanges = checkUndeployedChanges
			}
			if cmd.Flags().Changed("store-artifacts") {
				req.StoreArtifactsEnabled = storeArtifactsEnabled
			}

			project, err := client.UpdateProject(cliCtx.Background(), projectID, req)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(project, fmt.Sprintf("Updated project: %s", project.Name)))
			}
			env.Status("Updated project: %s", project.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&permalink, "permalink", "", "Project permalink")
	cmd.Flags().StringVar(&zone, "zone", "", "Project zone ID")
	cmd.Flags().StringVar(&emailNotifyOn, "email-notify-on", "", "When to send notifications (always, only_failures, never)")
	cmd.Flags().StringVar(&notificationEmail, "notification-email", "", "Custom notification email address")
	cmd.Flags().BoolVar(&notifyPusher, "notify-pusher", false, "Notify commit pusher on auto deploys")
	cmd.Flags().StringVar(&checkUndeployedChanges, "check-undeployed-changes", "", "Nightly scan for undeployed changes (true/false)")
	cmd.Flags().StringVar(&storeArtifactsEnabled, "store-artifacts", "", "Store .tar files for each deployment (true/false)")
	return cmd
}

func newProjectsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [permalink]",
		Short: "Delete a project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := resolveProjectArg(args)
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			if err := client.DeleteProject(cliCtx.Background(), projectID); err != nil {
				return err
			}

			cliCtx.Envelope.Status("Deleted project: %s", projectID)
			return nil
		},
	}
}

func newProjectsStarCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "star [permalink]",
		Aliases: []string{"fav", "unfav", "favourite", "unfavourite"},
		Short:   "Toggle project starred/favourite status",
		Args:    cobra.MaximumNArgs(1),
		ValidArgsFunction: completeProjectNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := resolveProjectArg(args)
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			if err := client.StarProject(cliCtx.Background(), projectID); err != nil {
				return err
			}

			cliCtx.Envelope.Status("Toggled favourite on project: %s", projectID)
			return nil
		},
	}
}

func newProjectsInsightsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "insights [permalink]",
		Short: "Show project deployment insights",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := resolveProjectArg(args)
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			insights, err := client.GetProjectInsights(cliCtx.Background(), projectID)
			if err != nil {
				return err
			}

			return cliCtx.Envelope.WriteJSON(output.NewResponse(insights,
				fmt.Sprintf("Insights for project: %s", projectID)))
		},
	}
}

func newProjectsUploadKeyCmd() *cobra.Command {
	var keyFile string

	cmd := &cobra.Command{
		Use:               "upload-key [permalink]",
		Short:             "Upload a custom public key for a project",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeProjectNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := resolveProjectArg(args)
			if err != nil {
				return err
			}

			if keyFile == "" {
				return &output.UserError{Message: "Public key file is required", Hint: "Use --key-file flag"}
			}

			keyData, err := os.ReadFile(keyFile)
			if err != nil {
				return fmt.Errorf("read key file: %w", err)
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			project, err := client.UploadProjectKey(cliCtx.Background(), projectID, string(keyData))
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(project, fmt.Sprintf("Uploaded key for project: %s", project.Name)))
			}
			env.Status("Uploaded custom key for project: %s", project.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to public key file (required)")
	return cmd
}

func newProjectsBadgeCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "badge [permalink]",
		Short:             "Get deployment status badge URL",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeProjectNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := resolveProjectArg(args)
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			badge, err := client.GetStatusBadge(cliCtx.Background(), projectID)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(
					map[string]string{"svg": string(badge)},
					fmt.Sprintf("Status badge for project: %s", projectID),
				))
			}

			// Write SVG to stdout for piping (e.g., dhq projects badge my-proj > badge.svg)
			fmt.Fprint(cmd.OutOrStdout(), string(badge))
			return nil
		},
	}
}

// resolveProjectArg gets the project ID from args or --project flag.
func resolveProjectArg(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	return cliCtx.RequireProject()
}
