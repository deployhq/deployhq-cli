package commands

import (
	"strconv"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage your own profile",
		Long:  `Your own user profile (the currently authenticated user).`,
	}
	cmd.AddCommand(
		newProfileGetCmd(),
		newProfileUpdateCmd(),
	)
	return cmd
}

func newProfileGetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "get", Short: "Show your profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			p, err := client.GetProfile(cliCtx.Background())
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(p, p.EmailAddress,
					output.Breadcrumb{Action: "update", Cmd: "dhq profile update"},
				))
			}
			env.WriteTable(
				[]string{"Field", "Value"},
				[][]string{
					{"Name", p.FirstName + " " + p.LastName},
					{"Email", p.EmailAddress},
					{"Identifier", p.Identifier},
					{"Time zone", p.TimeZone},
					{"Admin", strconv.FormatBool(p.AccountAdministrator)},
					{"Account", p.Account.Name},
					{"Package", p.Account.Package},
				},
			)
			return nil
		},
	}
}

func newProfileUpdateCmd() *cobra.Command {
	var (
		firstName, lastName, email, timeZone string
		alphabeticSort, promotionsEnabled    bool
	)
	cmd := &cobra.Command{
		Use: "update", Short: "Update your profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			req := sdk.ProfileUpdateRequest{}
			if cmd.Flags().Changed("first-name") {
				req.FirstName = &firstName
			}
			if cmd.Flags().Changed("last-name") {
				req.LastName = &lastName
			}
			if cmd.Flags().Changed("email") {
				req.EmailAddress = &email
			}
			if cmd.Flags().Changed("time-zone") {
				req.TimeZone = &timeZone
			}
			if cmd.Flags().Changed("alphabetic-sort") {
				req.AlphabeticSort = &alphabeticSort
			}
			if cmd.Flags().Changed("promotions-enabled") {
				req.PromotionsEnabled = &promotionsEnabled
			}
			if err := client.UpdateProfile(cliCtx.Background(), req); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(
					map[string]string{"status": "ok"},
					"Profile updated",
					output.Breadcrumb{Action: "get", Cmd: "dhq profile get"},
				))
			}
			env.Status("Profile updated")
			return nil
		},
	}
	cmd.Flags().StringVar(&firstName, "first-name", "", "First name")
	cmd.Flags().StringVar(&lastName, "last-name", "", "Last name")
	cmd.Flags().StringVar(&email, "email", "", "Email address")
	cmd.Flags().StringVar(&timeZone, "time-zone", "", "Time zone")
	cmd.Flags().BoolVar(&alphabeticSort, "alphabetic-sort", false, "Sort projects alphabetically")
	cmd.Flags().BoolVar(&promotionsEnabled, "promotions-enabled", false, "Receive promotional emails")
	return cmd
}
