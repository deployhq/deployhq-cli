package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newTeamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "teams",
		Short: "Manage teams (account-level permission groups)",
		Long: `Account-level permission/role groups. A team bundles a set of permission flags (admin, manage users, manage billing, manage agents, create projects, access all projects) and a list of member users who inherit them.

Teams are distinct from folders (which organise projects for display). Membership is synced with --user-ids on create/update; omit it to leave members untouched.

Note: when --admin is set, the server force-enables every other permission flag and grants access to all projects.`,
	}

	cmd.AddCommand(
		newTeamsListCmd(),
		newTeamsShowCmd(),
		newTeamsCreateCmd(),
		newTeamsUpdateCmd(),
		newTeamsDeleteCmd(),
	)

	return cmd
}

func newTeamsListCmd() *cobra.Command {
	var page, perPage int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List teams",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			teams, err := client.ListTeams(cliCtx.Background(), listOptsFromFlags(page, perPage))
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(teams, fmt.Sprintf("%d teams", len(teams)),
					output.Breadcrumb{Action: "show", Cmd: "dhq teams show <id>", Resource: "team"},
					output.Breadcrumb{Action: "create", Cmd: "dhq teams create <name>", Resource: "team"},
				))
			}

			if env.QuietMode {
				identifiers := make([]string, len(teams))
				for i, tm := range teams {
					identifiers[i] = tm.Identifier
				}
				env.WriteQuiet(identifiers)
				return nil
			}

			columns := []string{"Name", "Identifier", "Admin", "Members", "All-Projects"}
			rows := make([][]string, len(teams))
			for i, tm := range teams {
				rows[i] = []string{
					tm.Name,
					tm.Identifier,
					enabledLabel(tm.IsAdmin),
					fmt.Sprintf("%d", len(tm.Members)),
					enabledLabel(tm.AllProjectsAllowed),
				}
			}
			env.WriteTable(columns, rows)

			if len(teams) > 0 {
				env.Status("\nTip: dhq teams show %s", teams[0].Identifier)
			}
			return nil
		},
	}

	addPaginationFlags(cmd, &page, &perPage)
	return cmd
}

func newTeamsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show team details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			team, err := client.GetTeam(cliCtx.Background(), args[0])
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(team, fmt.Sprintf("Team: %s", team.Name),
					output.Breadcrumb{Action: "update", Cmd: fmt.Sprintf("dhq teams update %s", team.Identifier), Resource: "team", ID: team.Identifier},
					output.Breadcrumb{Action: "delete", Cmd: fmt.Sprintf("dhq teams delete %s", team.Identifier), Resource: "team", ID: team.Identifier},
				))
			}

			env.WriteTable([]string{"Field", "Value"}, [][]string{
				{"Name", team.Name},
				{"Identifier", team.Identifier},
				{"Admin", enabledLabel(team.IsAdmin)},
				{"Can manage users", enabledLabel(team.CanManageUsers)},
				{"Can manage billing", enabledLabel(team.CanManageBilling)},
				{"Can manage agents", enabledLabel(team.CanManageAgents)},
				{"Can create projects", enabledLabel(team.CanCreateProjects)},
				{"All projects allowed", enabledLabel(team.AllProjectsAllowed)},
				{"Members", fmt.Sprintf("%d", len(team.Members))},
			})

			if len(team.Members) > 0 {
				env.Status("\nMembers:")
				memberCols := []string{"Name", "Email", "Identifier"}
				memberRows := make([][]string, len(team.Members))
				for i, m := range team.Members {
					memberRows[i] = []string{
						fmt.Sprintf("%s %s", m.FirstName, m.LastName),
						m.EmailAddress,
						m.Identifier,
					}
				}
				env.WriteTable(memberCols, memberRows)
			}

			if len(team.ProjectAssignments) > 0 {
				env.Status("\nProject assignments:")
				paCols := []string{"Name", "Identifier", "Deploy-All", "Update-Config", "Manage-Config-Files"}
				paRows := make([][]string, len(team.ProjectAssignments))
				for i, pa := range team.ProjectAssignments {
					paRows[i] = []string{
						pa.Name, pa.Identifier,
						enabledLabel(pa.CanDeployAll), enabledLabel(pa.CanUpdateConfig), enabledLabel(pa.CanManageConfigFiles),
					}
				}
				env.WriteTable(paCols, paRows)
			}

			if len(team.ProjectExclusions) > 0 {
				env.Status("\nProject exclusions:")
				peCols := []string{"Name", "Identifier"}
				peRows := make([][]string, len(team.ProjectExclusions))
				for i, pe := range team.ProjectExclusions {
					peRows[i] = []string{pe.Name, pe.Identifier}
				}
				env.WriteTable(peCols, peRows)
			}

			env.Status("\nNext commands:")
			env.Status("  dhq teams update %s", team.Identifier)
			env.Status("  dhq teams delete %s", team.Identifier)
			return nil
		},
	}
}

// teamPermissionFlags holds the shared permission flags for create/update.
type teamPermissionFlags struct {
	admin             bool
	canManageUsers    bool
	canManageBilling  bool
	canManageAgents   bool
	canCreateProjects bool
	allProjects       bool
	userIDs           []int
}

func addTeamPermissionFlags(cmd *cobra.Command, f *teamPermissionFlags) {
	cmd.Flags().BoolVar(&f.admin, "admin", false, "Grant admin (server force-enables all other permissions)")
	cmd.Flags().BoolVar(&f.canManageUsers, "can-manage-users", false, "Allow managing users")
	cmd.Flags().BoolVar(&f.canManageBilling, "can-manage-billing", false, "Allow managing billing")
	cmd.Flags().BoolVar(&f.canManageAgents, "can-manage-agents", false, "Allow managing build agents")
	cmd.Flags().BoolVar(&f.canCreateProjects, "can-create-projects", false, "Allow creating projects")
	cmd.Flags().BoolVar(&f.allProjects, "all-projects", false, "Grant access to all projects")
	cmd.Flags().IntSliceVar(&f.userIDs, "user-ids", nil, "Sync team members to these user IDs (comma-separated); omit to leave membership untouched")
}

func newTeamsCreateCmd() *cobra.Command {
	var f teamPermissionFlags

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a team",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if name == "" {
				return &output.UserError{Message: "Name is required", Hint: "Usage: dhq teams create <name>"}
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			req := sdk.TeamCreateRequest{
				Name:               name,
				IsAdmin:            f.admin,
				CanManageUsers:     f.canManageUsers,
				CanManageBilling:   f.canManageBilling,
				CanManageAgents:    f.canManageAgents,
				CanCreateProjects:  f.canCreateProjects,
				AllProjectsAllowed: f.allProjects,
			}
			if cmd.Flags().Changed("user-ids") {
				req.UserIDs = f.userIDs
			}

			team, err := client.CreateTeam(cliCtx.Background(), req)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(team, fmt.Sprintf("Created team: %s", team.Name),
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("dhq teams show %s", team.Identifier), Resource: "team", ID: team.Identifier},
				))
			}
			env.Status("Created team: %s (%s)", team.Name, team.Identifier)
			return nil
		},
	}

	addTeamPermissionFlags(cmd, &f)
	return cmd
}

func newTeamsUpdateCmd() *cobra.Command {
	var f teamPermissionFlags
	var name string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a team",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			// Only send fields the user explicitly set, so unset flags don't
			// clobber existing values.
			var req sdk.TeamUpdateRequest
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("admin") {
				req.IsAdmin = &f.admin
			}
			if cmd.Flags().Changed("can-manage-users") {
				req.CanManageUsers = &f.canManageUsers
			}
			if cmd.Flags().Changed("can-manage-billing") {
				req.CanManageBilling = &f.canManageBilling
			}
			if cmd.Flags().Changed("can-manage-agents") {
				req.CanManageAgents = &f.canManageAgents
			}
			if cmd.Flags().Changed("can-create-projects") {
				req.CanCreateProjects = &f.canCreateProjects
			}
			if cmd.Flags().Changed("all-projects") {
				req.AllProjectsAllowed = &f.allProjects
			}
			if cmd.Flags().Changed("user-ids") {
				// Pointer so an explicit empty list (--user-ids "") clears all
				// members instead of being dropped by omitempty. IntSliceVar
				// yields a non-nil empty slice for "", not nil.
				ids := f.userIDs
				if ids == nil {
					ids = []int{}
				}
				req.UserIDs = &ids
			}

			team, err := client.UpdateTeam(cliCtx.Background(), args[0], req)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(team, fmt.Sprintf("Updated team: %s", team.Name),
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("dhq teams show %s", team.Identifier), Resource: "team", ID: team.Identifier},
				))
			}
			env.Status("Updated team: %s", team.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New team name")
	addTeamPermissionFlags(cmd, &f)
	return cmd
}

func newTeamsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a team",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			if err := client.DeleteTeam(cliCtx.Background(), args[0]); err != nil {
				return err
			}
			cliCtx.Envelope.Status("Deleted team: %s", args[0])
			return nil
		},
	}
}
