package commands

import (
	"fmt"
	"strconv"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newUsersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage account users",
		Long:  `Account-level team members. Users are keyed by their string identifier.`,
	}
	cmd.AddCommand(
		newUsersListCmd(),
		newUsersShowCmd(),
		newUsersCreateCmd(),
		newUsersUpdateCmd(),
		newUsersDeleteCmd(),
		newUsersResendInvitationCmd(),
	)
	return cmd
}

func newUsersListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			users, err := client.ListUsers(cliCtx.Background(), nil)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(users, fmt.Sprintf("%d users", len(users)),
					output.Breadcrumb{Action: "show", Cmd: "dhq users show <identifier>"},
					output.Breadcrumb{Action: "create", Cmd: "dhq users create --email <email> --first-name <name> --last-name <name>"},
					output.Breadcrumb{Action: "delete", Cmd: "dhq users delete <identifier>"},
				))
			}
			if env.QuietMode {
				identifiers := make([]string, len(users))
				for i, u := range users {
					identifiers[i] = u.Identifier
				}
				env.WriteQuiet(identifiers)
				return nil
			}
			rows := make([][]string, len(users))
			for i, u := range users {
				rows[i] = []string{
					fmt.Sprintf("%s %s", u.FirstName, u.LastName),
					u.EmailAddress,
					u.Identifier,
					strconv.FormatBool(u.AccountAdministrator),
					strconv.FormatBool(u.Activated),
				}
			}
			env.WriteTable([]string{"Name", "Email", "Identifier", "Admin", "Activated"}, rows)
			return nil
		},
	}
}

func newUsersShowCmd() *cobra.Command {
	return &cobra.Command{
		Use: "show <identifier>", Short: "Show a user", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			u, err := client.GetUser(cliCtx.Background(), args[0])
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(u, fmt.Sprintf("%s %s", u.FirstName, u.LastName),
					output.Breadcrumb{Action: "update", Cmd: "dhq users update <identifier>", Resource: "user", ID: u.Identifier},
					output.Breadcrumb{Action: "delete", Cmd: "dhq users delete <identifier>", Resource: "user", ID: u.Identifier},
				))
			}
			env.WriteTable(
				[]string{"Field", "Value"},
				[][]string{
					{"Name", fmt.Sprintf("%s %s", u.FirstName, u.LastName)},
					{"Email", u.EmailAddress},
					{"Identifier", u.Identifier},
					{"Time zone", u.TimeZone},
					{"Admin", strconv.FormatBool(u.AccountAdministrator)},
					{"Activated", strconv.FormatBool(u.Activated)},
					{"All projects", strconv.FormatBool(u.AllProjectsAllowed)},
				},
			)
			return nil
		},
	}
}

// userRequestFromFlags builds a UserRequest from the flags the caller actually
// set. Only changed flags are included so an update never clobbers untouched
// attributes.
func userRequestFromFlags(cmd *cobra.Command, firstName, lastName, email, timeZone *string, allProjects, admin, manageUsers, manageBilling, manageAgents, createProjects *bool) sdk.UserRequest {
	req := sdk.UserRequest{}
	if cmd.Flags().Changed("first-name") {
		req.FirstName = firstName
	}
	if cmd.Flags().Changed("last-name") {
		req.LastName = lastName
	}
	if cmd.Flags().Changed("email") {
		req.EmailAddress = email
	}
	if cmd.Flags().Changed("time-zone") {
		req.TimeZone = timeZone
	}
	if cmd.Flags().Changed("all-projects") {
		req.AllProjectsAllowed = allProjects
	}
	if cmd.Flags().Changed("admin") {
		req.AccountAdministrator = admin
	}
	if cmd.Flags().Changed("can-manage-users") {
		req.CanManageUsers = manageUsers
	}
	if cmd.Flags().Changed("can-manage-billing") {
		req.CanManageBilling = manageBilling
	}
	if cmd.Flags().Changed("can-manage-agents") {
		req.CanManageAgents = manageAgents
	}
	if cmd.Flags().Changed("can-create-projects") {
		req.CanCreateProjects = createProjects
	}
	return req
}

func addUserFlags(cmd *cobra.Command, firstName, lastName, email, timeZone *string, allProjects, admin, manageUsers, manageBilling, manageAgents, createProjects *bool) {
	cmd.Flags().StringVar(firstName, "first-name", "", "First name")
	cmd.Flags().StringVar(lastName, "last-name", "", "Last name")
	cmd.Flags().StringVar(email, "email", "", "Email address")
	cmd.Flags().StringVar(timeZone, "time-zone", "", "Time zone")
	cmd.Flags().BoolVar(allProjects, "all-projects", false, "Grant access to all projects")
	cmd.Flags().BoolVar(admin, "admin", false, "Account administrator (admin only)")
	cmd.Flags().BoolVar(manageUsers, "can-manage-users", false, "Can manage users (admin only)")
	cmd.Flags().BoolVar(manageBilling, "can-manage-billing", false, "Can manage billing (admin only)")
	cmd.Flags().BoolVar(manageAgents, "can-manage-agents", false, "Can manage agents (admin only)")
	cmd.Flags().BoolVar(createProjects, "can-create-projects", false, "Can create projects (admin only)")
}

func newUsersCreateCmd() *cobra.Command {
	var (
		firstName, lastName, email, timeZone                                         string
		allProjects, admin, manageUsers, manageBilling, manageAgents, createProjects bool
	)
	cmd := &cobra.Command{
		Use: "create", Short: "Create (invite) a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" {
				return &output.UserError{Message: "--email is required"}
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			req := userRequestFromFlags(cmd, &firstName, &lastName, &email, &timeZone,
				&allProjects, &admin, &manageUsers, &manageBilling, &manageAgents, &createProjects)
			u, err := client.CreateUser(cliCtx.Background(), req)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(u, fmt.Sprintf("Invited: %s", u.EmailAddress),
					output.Breadcrumb{Action: "resend-invitation", Cmd: "dhq users resend-invitation <identifier>", Resource: "user", ID: u.Identifier},
					output.Breadcrumb{Action: "update", Cmd: "dhq users update <identifier>", Resource: "user", ID: u.Identifier},
				))
			}
			env.Status("Invited user: %s (%s)", u.EmailAddress, u.Identifier)
			return nil
		},
	}
	addUserFlags(cmd, &firstName, &lastName, &email, &timeZone,
		&allProjects, &admin, &manageUsers, &manageBilling, &manageAgents, &createProjects)
	return cmd
}

func newUsersUpdateCmd() *cobra.Command {
	var (
		firstName, lastName, email, timeZone                                         string
		allProjects, admin, manageUsers, manageBilling, manageAgents, createProjects bool
	)
	cmd := &cobra.Command{
		Use: "update <identifier>", Short: "Update a user", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			req := userRequestFromFlags(cmd, &firstName, &lastName, &email, &timeZone,
				&allProjects, &admin, &manageUsers, &manageBilling, &manageAgents, &createProjects)
			u, err := client.UpdateUser(cliCtx.Background(), args[0], req)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(u, fmt.Sprintf("Updated: %s", u.EmailAddress),
					output.Breadcrumb{Action: "show", Cmd: "dhq users show <identifier>", Resource: "user", ID: u.Identifier},
				))
			}
			env.Status("Updated user: %s (%s)", u.EmailAddress, u.Identifier)
			return nil
		},
	}
	addUserFlags(cmd, &firstName, &lastName, &email, &timeZone,
		&allProjects, &admin, &manageUsers, &manageBilling, &manageAgents, &createProjects)
	return cmd
}

func newUsersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use: "delete <identifier>", Short: "Delete a user", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.DeleteUser(cliCtx.Background(), args[0]); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(
					map[string]string{"identifier": args[0], "status": "deleted"},
					fmt.Sprintf("Deleted: %s", args[0]),
				))
			}
			env.Status("Deleted user: %s", args[0])
			return nil
		},
	}
}

func newUsersResendInvitationCmd() *cobra.Command {
	return &cobra.Command{
		Use: "resend-invitation <identifier>", Short: "Resend a user's activation invitation", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.ResendUserInvitation(cliCtx.Background(), args[0]); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(
					map[string]string{"identifier": args[0], "status": "ok"},
					fmt.Sprintf("Invitation resent: %s", args[0]),
				))
			}
			env.Status("Resent invitation to user: %s", args[0])
			return nil
		},
	}
}
