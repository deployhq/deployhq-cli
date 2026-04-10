package commands

import (
	"os"

	"github.com/deployhq/deployhq-cli/internal/auth"
	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check CLI configuration and connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			checks := []doctorCheck{}

			// Check 1: Auth
			creds, err := auth.Load()
			if err != nil {
				checks = append(checks, doctorCheck{"Authentication", "fail", "Not logged in. Run 'dhq auth login'."})
			} else {
				checks = append(checks, doctorCheck{"Authentication", "ok", "Logged in as " + creds.Email})
			}

			// Check 2: Account configured
			host := "deployhq.com"
			if cliCtx.Config.Host != "" {
				host = cliCtx.Config.Host
			}
			if cliCtx.Config.Account != "" {
				checks = append(checks, doctorCheck{"Account", "ok", cliCtx.Config.Account + "." + host})
			} else if creds != nil && creds.Account != "" {
				checks = append(checks, doctorCheck{"Account", "ok", creds.Account + "." + host + " (from auth store)"})
			} else {
				checks = append(checks, doctorCheck{"Account", "fail", "No account configured. Set DEPLOYHQ_ACCOUNT or run 'dhq config set account <name>'."})
			}

			// Check 3: Project config
			if cliCtx.Config.Project != "" {
				checks = append(checks, doctorCheck{"Project", "ok", cliCtx.Config.Project + " (source: " + cliCtx.Config.Sources["project"] + ")"})
			} else {
				checks = append(checks, doctorCheck{"Project", "warn", "No default project. Use --project flag or .deployhq.toml."})
			}

			// Check 4: Project config file
			projectCfg := config.ProjectConfigPath()
			if _, err := os.Stat(projectCfg); err == nil {
				checks = append(checks, doctorCheck{"Project config", "ok", projectCfg})
			} else {
				checks = append(checks, doctorCheck{"Project config", "info", "No .deployhq.toml found. Run 'dhq config init' to create one."})
			}

			// Check 5: Global config
			globalCfg := config.GlobalConfigPath()
			if _, err := os.Stat(globalCfg); err == nil {
				checks = append(checks, doctorCheck{"Global config", "ok", globalCfg})
			} else {
				checks = append(checks, doctorCheck{"Global config", "info", "No global config. Run 'dhq config init --global'."})
			}

			// Check 6: API connectivity
			client, err := cliCtx.Client()
			if err != nil {
				checks = append(checks, doctorCheck{"API connectivity", "fail", err.Error()})
			} else {
				_, err := client.ListProjects(cliCtx.Background(), nil)
				if err != nil {
					checks = append(checks, doctorCheck{"API connectivity", "fail", err.Error()})
				} else {
					checks = append(checks, doctorCheck{"API connectivity", "ok", "Connected"})
				}
			}

			// Output
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(checks, "Health check complete"))
			}

			for _, c := range checks {
				icon := "?"
				switch c.Status {
				case "ok":
					icon = "+"
				case "warn":
					icon = "!"
				case "fail":
					icon = "x"
				case "info":
					icon = "-"
				}
				env.Status("[%s] %s: %s", icon, c.Name, c.Detail)
			}
			return nil
		},
	}
}

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warn, fail, info
	Detail string `json:"detail"`
}
