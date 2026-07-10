package commands

import (
	"strconv"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage the current account",
		Long:  `The current DeployHQ account. A singular resource — there is no list.`,
	}
	cmd.AddCommand(
		newAccountGetCmd(),
		newAccountUpdateCmd(),
		newAccountBillingCmd(),
	)
	return cmd
}

func newAccountGetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "get", Short: "Show the current account",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			a, err := client.GetAccount(cliCtx.Background())
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(a, a.Name,
					output.Breadcrumb{Action: "update", Cmd: "dhq account update"},
					output.Breadcrumb{Action: "billing", Cmd: "dhq account billing"},
				))
			}
			limit := "unlimited"
			if a.ProjectLimit != nil {
				limit = strconv.Itoa(*a.ProjectLimit)
			}
			env.WriteTable(
				[]string{"Field", "Value"},
				[][]string{
					{"Name", a.Name},
					{"Permalink", a.Permalink},
					{"Time zone", a.TimeZone},
					{"Package", a.Package},
					{"Trialling", strconv.FormatBool(a.Trialling)},
					{"Suspended", strconv.FormatBool(a.Suspended)},
					{"Projects", strconv.Itoa(a.ProjectCount)},
					{"Project limit", limit},
					{"AI features disabled", strconv.FormatBool(a.AIFeaturesDisabled)},
				},
			)
			return nil
		},
	}
}

func newAccountUpdateCmd() *cobra.Command {
	var (
		name, permalink, timeZone, cname                            string
		ipRestricted, aiDisabled, twoFactorRequired, strongPassword bool
	)
	cmd := &cobra.Command{
		Use: "update", Short: "Update the current account",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			req := sdk.AccountUpdateRequest{}
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("permalink") {
				req.Permalink = &permalink
			}
			if cmd.Flags().Changed("time-zone") {
				req.TimeZone = &timeZone
			}
			if cmd.Flags().Changed("cname") {
				req.CName = &cname
			}
			if cmd.Flags().Changed("ip-restricted") {
				req.IPRestricted = &ipRestricted
			}
			if cmd.Flags().Changed("ai-features-disabled") {
				req.AIFeaturesDisabled = &aiDisabled
			}
			if cmd.Flags().Changed("two-factor-required") {
				req.TwoFactorAuthRequired = &twoFactorRequired
			}
			if cmd.Flags().Changed("strong-password-required") {
				req.StrongPasswordRequired = &strongPassword
			}
			a, err := client.UpdateAccount(cliCtx.Background(), req)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(a, "Updated account: "+a.Name,
					output.Breadcrumb{Action: "get", Cmd: "dhq account get"},
				))
			}
			env.Status("Updated account: %s", a.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Account name")
	cmd.Flags().StringVar(&permalink, "permalink", "", "Account permalink")
	cmd.Flags().StringVar(&timeZone, "time-zone", "", "Time zone")
	cmd.Flags().StringVar(&cname, "cname", "", "Custom CNAME")
	cmd.Flags().BoolVar(&ipRestricted, "ip-restricted", false, "Restrict access by IP")
	cmd.Flags().BoolVar(&aiDisabled, "ai-features-disabled", false, "Disable AI features")
	cmd.Flags().BoolVar(&twoFactorRequired, "two-factor-required", false, "Require two-factor authentication")
	cmd.Flags().BoolVar(&strongPassword, "strong-password-required", false, "Require strong passwords")
	return cmd
}

func newAccountBillingCmd() *cobra.Command {
	return &cobra.Command{
		Use: "billing", Short: "Show billing status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			b, err := client.GetBillingStatus(cliCtx.Background())
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(b, "Billing status",
					output.Breadcrumb{Action: "get", Cmd: "dhq account get"},
				))
			}
			rows := [][]string{
				{"Package", b.Package},
				{"Frequency", strconv.Itoa(b.Frequency)},
				{"Trialling", strconv.FormatBool(b.Trialling)},
				{"Suspended", strconv.FormatBool(b.Suspended)},
			}
			if b.ScheduledChange != nil {
				rows = append(rows,
					[]string{"Scheduled package", b.ScheduledChange.TargetPackage},
					[]string{"Scheduled frequency", strconv.Itoa(b.ScheduledChange.TargetFrequency)},
					[]string{"Effective at", b.ScheduledChange.EffectiveAt},
				)
			}
			env.WriteTable([]string{"Field", "Value"}, rows)
			return nil
		},
	}
}
