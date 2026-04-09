package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newServersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "servers",
		Aliases: []string{"server", "srv"},
		Short:   "Manage servers",
	}

	cmd.AddCommand(
		newServersListCmd(),
		newServersShowCmd(),
		newServersCreateCmd(),
		newServersUpdateCmd(),
		newServersDeleteCmd(),
		newServersResetHostKeyCmd(),
	)

	return cmd
}

func newServersListCmd() *cobra.Command {
	var page, perPage int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List servers in a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			servers, err := client.ListServers(cliCtx.Background(), projectID, listOptsFromFlags(page, perPage))
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(servers,
					fmt.Sprintf("%d servers", len(servers)),
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("dhq servers show <id> -p %s", projectID)},
					output.Breadcrumb{Action: "deploy", Cmd: fmt.Sprintf("dhq deploy -p %s", projectID)},
				))
			}

			columns := []string{"Name", "Identifier", "Protocol", "Branch", "Enabled"}
			rows := make([][]string, len(servers))
			for i, s := range servers {
				enabled := "yes"
				if !s.Enabled {
					enabled = "no"
				}
				rows[i] = []string{s.Name, s.Identifier, s.ProtocolType, s.Branch, enabled}
			}
			env.WriteTable(columns, rows)

			if len(servers) > 0 {
				env.Status("\nTip: dhq deploy -p %s -s %s", projectID, servers[0].Identifier)
			}
			return nil
		},
	}

	addPaginationFlags(cmd, &page, &perPage)
	return cmd
}

func newServersShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "show <identifier>",
		Short:             "Show server details",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			server, err := client.GetServer(cliCtx.Background(), projectID, args[0])
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(server,
					fmt.Sprintf("Server: %s", server.Name),
					output.Breadcrumb{Action: "deploy", Cmd: fmt.Sprintf("dhq deploy -p %s", projectID)},
					output.Breadcrumb{Action: "reset-host-key", Cmd: fmt.Sprintf("dhq servers reset-host-key %s -p %s", server.Identifier, projectID)},
				))
			}

			enabled := "yes"
			if !server.Enabled {
				enabled = "no"
			}
			env.WriteTable([]string{"Field", "Value"}, [][]string{
				{"Name", server.Name},
				{"Identifier", server.Identifier},
				{"Protocol", server.ProtocolType},
				{"Path", server.ServerPath},
				{"Branch", server.Branch},
				{"Environment", server.Environment},
				{"Enabled", output.ColorStatus(enabled)},
				{"Last Revision", server.LastRevision},
			})

			s := server.Identifier
			env.Status("\nNext commands:")
			env.Status("  dhq deploy -p %s -s %s", projectID, s)
			env.Status("  dhq servers update %s -p %s", s, projectID)
			env.Status("  dhq env-vars list -p %s", projectID)
			return nil
		},
	}
}

func newServersCreateCmd() *cobra.Command {
	var name, protocolType, serverPath, environment string
	// SSH / FTP / FTPS / Rsync
	var hostname, username, password, globalKeyPairID string
	var port int
	var useSSHKeys, installKey bool
	// S3 / S3-Compatible
	var bucketName, accessKeyID, secretAccessKey, customEndpoint string
	// DigitalOcean
	var personalAccessToken, dropletName string
	// Hetzner Cloud
	var apiToken, hetznerServerName string
	// Heroku
	var appName, apiKeyHeroku string
	// Netlify
	var siteID, accessToken string
	// Shopify
	var storeURL, themeName string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return &output.UserError{Message: "Server name is required", Hint: "Use --name flag"}
			}
			if protocolType == "" {
				return &output.UserError{
					Message: "Protocol type is required",
					Hint:    "Use --protocol-type with one of: ssh, ftp, ftps, rsync, s3, s3_compatible, digitalocean, hetzner_cloud, heroku, netlify, shopify",
				}
			}

			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			req := sdk.ServerCreateRequest{
				Name:         name,
				ProtocolType: protocolType,
				ServerPath:   serverPath,
				Environment:  environment,
				// SSH / FTP / FTPS / Rsync
				Hostname: hostname,
				Username: username,
				Password: password,
				// S3
				BucketName:     bucketName,
				AccessKeyID:    accessKeyID,
				SecretAccessKey: secretAccessKey,
				// S3-Compatible
				CustomEndpoint: customEndpoint,
				// DigitalOcean
				PersonalAccessToken: personalAccessToken,
				DropletName:         dropletName,
				// Hetzner Cloud
				APIToken:          apiToken,
				HetznerServerName: hetznerServerName,
				// Heroku
				AppName:      appName,
				APIKeyHeroku: apiKeyHeroku,
				// Netlify
				SiteID:      siteID,
				AccessToken: accessToken,
				// Shopify
				StoreURL:  storeURL,
				ThemeName: themeName,
			}
			if cmd.Flags().Changed("port") {
				req.Port = &port
			}
			if cmd.Flags().Changed("use-ssh-keys") {
				req.UseSSHKeys = &useSSHKeys
			}
			if globalKeyPairID != "" {
				req.GlobalKeyPairID = globalKeyPairID
			}

			env := cliCtx.Envelope

			var server *sdk.Server
			for {
				var err error
				server, err = client.CreateServer(cliCtx.Background(), projectID, req)
				if err == nil {
					break
				}

				// Only handle SSH auth failures with key-based auth
				isAuthError := strings.Contains(strings.ToLower(err.Error()), "authentication failed")
				if !isAuthError || !useSSHKeys {
					return err
				}

				// Resolve the public key for display / installation
				var publicKey string
				if globalKeyPairID != "" {
					keys, kerr := client.ListSSHKeys(cliCtx.Background(), nil)
					if kerr == nil {
						for _, k := range keys {
							if k.Identifier == globalKeyPairID {
								publicKey = k.PublicKey
								break
							}
						}
					}
				}

				// --install-key: attempt to install the key on the server automatically
				if installKey && publicKey != "" && hostname != "" && username != "" {
					sshPort := 22
					if cmd.Flags().Changed("port") {
						sshPort = port
					}
					env.Warn("SSH authentication failed — attempting to install the deploy key on the server...")
					if ierr := installSSHKey(env, hostname, sshPort, username, publicKey); ierr == nil {
						env.Status("Key installed successfully. Retrying server creation...")
						continue // retry CreateServer
					}
					env.Warn("Could not install the key automatically.")
				}

				// Non-interactive: return error with actionable hint
				if !env.IsTTY || env.JSONMode {
					if publicKey != "" {
						return &output.UserError{
							Message: err.Error(),
							Hint:    fmt.Sprintf("Add this key to %s@%s:~/.ssh/authorized_keys:\n  %s", username, hostname, publicKey),
						}
					}
					return err
				}

				// Interactive fallback: show key and offer manual retry
				env.Warn("SSH authentication failed.")
				if publicKey != "" {
					env.Status("")
					env.Status("Make sure this key is in your server's ~/.ssh/authorized_keys:")
					env.Status("")
					env.Status("  %s", publicKey)
				}
				env.Status("")
				fmt.Fprint(env.Stderr, "Retry? (Y/n): ") //nolint:errcheck
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer == "n" || answer == "no" {
					return err
				}
			}

			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(server, fmt.Sprintf("Created server: %s", server.Name)))
			}
			env.Status("Created server: %s (%s)", server.Name, server.Identifier)
			return nil
		},
	}

	// Common flags
	cmd.Flags().StringVar(&name, "name", "", "Server name (required)")
	cmd.Flags().StringVar(&protocolType, "protocol-type", "", "Protocol (required): ssh, ftp, ftps, rsync, s3, s3_compatible, digitalocean, hetzner_cloud, heroku, netlify, shopify")
	cmd.Flags().StringVar(&serverPath, "path", "", "Server path")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment name")

	// SSH / FTP / FTPS / Rsync
	cmd.Flags().StringVar(&hostname, "hostname", "", "Server hostname or IP address (ssh, ftp, ftps, rsync)")
	cmd.Flags().StringVar(&username, "username", "", "Server username (ssh, ftp, ftps, rsync, digitalocean, hetzner_cloud)")
	cmd.Flags().StringVar(&password, "password", "", "Server password (ssh, ftp, ftps)")
	cmd.Flags().IntVar(&port, "port", 0, "Server port (ssh, ftp, ftps, rsync)")
	cmd.Flags().BoolVar(&useSSHKeys, "use-ssh-keys", false, "Use SSH key authentication (ssh, rsync)")
	cmd.Flags().StringVar(&globalKeyPairID, "global-key-pair-id", "", "Global SSH key pair identifier (ssh, rsync)")
	cmd.Flags().BoolVar(&installKey, "install-key", false, "Attempt to install the SSH key on the server via ssh-copy-id")

	// S3
	cmd.Flags().StringVar(&bucketName, "bucket-name", "", "S3 bucket name (s3, s3_compatible)")
	cmd.Flags().StringVar(&accessKeyID, "access-key-id", "", "AWS access key ID (s3, s3_compatible)")
	cmd.Flags().StringVar(&secretAccessKey, "secret-access-key", "", "AWS secret access key (s3, s3_compatible)")

	// S3-Compatible
	cmd.Flags().StringVar(&customEndpoint, "custom-endpoint", "", "Custom S3 endpoint URL (s3_compatible)")

	// DigitalOcean
	cmd.Flags().StringVar(&personalAccessToken, "personal-access-token", "", "DigitalOcean personal access token (digitalocean)")
	cmd.Flags().StringVar(&dropletName, "droplet-name", "", "DigitalOcean droplet name (digitalocean)")

	// Hetzner Cloud
	cmd.Flags().StringVar(&apiToken, "api-token", "", "Hetzner Cloud API token (hetzner_cloud)")
	cmd.Flags().StringVar(&hetznerServerName, "hetzner-server-name", "", "Hetzner server name (hetzner_cloud)")

	// Heroku
	cmd.Flags().StringVar(&appName, "app-name", "", "Heroku app name (heroku)")
	cmd.Flags().StringVar(&apiKeyHeroku, "api-key", "", "Heroku API key (heroku)")

	// Netlify
	cmd.Flags().StringVar(&siteID, "site-id", "", "Netlify site ID (netlify)")
	cmd.Flags().StringVar(&accessToken, "access-token", "", "Access token (netlify, shopify)")

	// Shopify
	cmd.Flags().StringVar(&storeURL, "store-url", "", "Shopify store URL (shopify)")
	cmd.Flags().StringVar(&themeName, "theme-name", "", "Shopify theme name (shopify)")

	return cmd
}

func newServersUpdateCmd() *cobra.Command {
	var name, serverPath, environment string

	cmd := &cobra.Command{
		Use:               "update <identifier>",
		Short:             "Update a server",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			req := sdk.ServerUpdateRequest{Name: name, ServerPath: serverPath, Environment: environment}
			server, err := client.UpdateServer(cliCtx.Background(), projectID, args[0], req)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(server, fmt.Sprintf("Updated server: %s", server.Name)))
			}
			env.Status("Updated server: %s", server.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Server name")
	cmd.Flags().StringVar(&serverPath, "path", "", "Server path")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment name")
	return cmd
}

func newServersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "delete <identifier>",
		Short:             "Delete a server",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			if err := client.DeleteServer(cliCtx.Background(), projectID, args[0]); err != nil {
				return err
			}
			cliCtx.Envelope.Status("Deleted server: %s", args[0])
			return nil
		},
	}
}

func newServersResetHostKeyCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "reset-host-key <identifier>",
		Short:             "Reset SSH host key for a server",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			if err := client.ResetServerHostKey(cliCtx.Background(), projectID, args[0]); err != nil {
				return err
			}
			cliCtx.Envelope.Status("Reset host key for server: %s", args[0])
			return nil
		},
	}
}
