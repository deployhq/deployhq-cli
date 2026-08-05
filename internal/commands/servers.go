package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// atomicStrategyCopyRelease is DeployHQ's own default. `servers create` sends
// it explicitly when --atomic is enabled without a strategy, so the payload the
// CLI produces is deterministic rather than dependent on a backend default.
const atomicStrategyCopyRelease = "copy_release"

// atomicStrategies are the values the DeployHQ backend accepts.
var atomicStrategies = []string{atomicStrategyCopyRelease, "copy_cache"}

// serverDeploymentFlags binds the deployment-configuration flags shared by
// `dhq servers create` and `dhq servers update`, so the two commands cannot
// drift apart.
//
// Every field is applied only when its flag was explicitly supplied. That is
// what keeps `servers update` from mutating settings the operator never named,
// and it is also why the booleans and the retention count reach the SDK as
// pointers: an explicit `--auto-deploy=false` must serialise as `false`, while
// an omitted one must not appear in the request body at all.
//
// The CLI deliberately does NOT replicate the backend's protocol/account policy
// for atomic deployments. Those checks live server-side and their structured
// errors pass through untouched.
type serverDeploymentFlags struct {
	branch          string
	autoDeploy      bool
	atomic          bool
	atomicStrategy  string
	atomicRetention int

	// flags is the set the values were registered on; it answers "was this
	// flag actually supplied?" via Changed().
	flags *pflag.FlagSet
}

// register adds the five flags to cmd and records the flag set.
func (f *serverDeploymentFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.branch, "branch", "",
		`Branch this server deploys from, e.g. main or staging. `+
			`Pass --branch "" to unpin the server and fall back to the repository default. `+
			`Has no effect while the server belongs to a server group — the group's branch wins`)
	cmd.Flags().BoolVar(&f.autoDeploy, "auto-deploy", false,
		"Deploy automatically when new commits reach the server's branch. "+
			"The backend suppresses auto-deploy while the server belongs to a server group. "+
			"Use --auto-deploy=false to turn it off")
	cmd.Flags().BoolVar(&f.atomic, "atomic", false,
		"Enable zero-downtime (atomic) deployments. Must be set before the server's first deployment — "+
			"the backend rejects any change afterwards. Requires a supported protocol "+
			"(ssh, rsync, digitalocean, hetzner_cloud, managed_vps) and an account with "+
			"atomic deployments enabled. Use --atomic=false to turn it off")
	cmd.Flags().StringVar(&f.atomicStrategy, "atomic-strategy", "",
		"Atomic release strategy: copy_release or copy_cache (DeployHQ default: copy_release)")
	cmd.Flags().IntVar(&f.atomicRetention, "atomic-retention", 0,
		"Number of past atomic releases to keep; must be 1 or greater (DeployHQ default: 3)")

	f.flags = cmd.Flags()
}

// supplied reports whether the named flag was explicitly given on the command
// line. An unregistered flag set (helper never registered) counts as "no".
func (f *serverDeploymentFlags) supplied(name string) bool {
	return f.flags != nil && f.flags.Changed(name)
}

// validate runs the purely local checks. It must be called before the command
// resolves a project or builds an API client, so a malformed invocation fails
// with zero network access and no credentials.
func (f *serverDeploymentFlags) validate() error {
	if f.supplied("atomic-strategy") {
		valid := false
		for _, s := range atomicStrategies {
			if f.atomicStrategy == s {
				valid = true
				break
			}
		}
		if !valid {
			return &output.UserError{
				Message: fmt.Sprintf("Invalid --atomic-strategy %q", f.atomicStrategy),
				Hint:    "Use one of: " + strings.Join(atomicStrategies, ", "),
			}
		}
	}

	if f.supplied("atomic-retention") && f.atomicRetention < 1 {
		return &output.UserError{
			Message: fmt.Sprintf("Invalid --atomic-retention %d", f.atomicRetention),
			Hint:    "Retention is the number of past releases to keep, so it must be 1 or greater (DeployHQ default: 3).",
		}
	}

	return nil
}

// applyToCreate copies the supplied flags onto a create request.
//
// Create additionally pins the strategy to copy_release when --atomic is
// enabled without one, matching the backend default and making the emitted
// payload deterministic. Update deliberately does NOT do this.
func (f *serverDeploymentFlags) applyToCreate(req *sdk.ServerCreateRequest) {
	if f.supplied("branch") {
		v := f.branch
		req.Branch = &v
	}
	if f.supplied("auto-deploy") {
		v := f.autoDeploy
		req.AutoDeploy = &v
	}
	if f.supplied("atomic") {
		v := f.atomic
		req.Atomic = &v
	}
	if f.supplied("atomic-strategy") {
		req.AtomicStrategy = f.atomicStrategy
	}
	if f.supplied("atomic-retention") {
		v := f.atomicRetention
		req.AtomicRetention = &v
	}

	if req.Atomic != nil && *req.Atomic && req.AtomicStrategy == "" {
		req.AtomicStrategy = atomicStrategyCopyRelease
	}
}

// branchIsDormant reports whether a branch the operator just set will have no
// effect on deployments because the server belongs to a server group.
//
// The backend resolves a server's branch as
// `server_group&.branch&.presence || branch.presence || repository.branch`, and
// grouped servers are excluded from auto-deployment altogether (the group is
// the deployable). So a branch stored on a grouped server is inert — the write
// succeeds and is echoed back, but nothing ever deploys from it. The Rails UI
// sidesteps this by hiding the field for grouped servers; the API does not, so
// the CLI says it out loud instead of letting the operator believe it took.
func branchIsDormant(branchSupplied bool, server *sdk.Server) bool {
	if !branchSupplied || server == nil {
		return false
	}
	return server.ServerGroupIdentifier != nil && *server.ServerGroupIdentifier != ""
}

// warnIfBranchDormant emits the dormant-branch warning on stderr, keeping
// stdout pure data.
func warnIfBranchDormant(env *output.Envelope, branchSupplied bool, server *sdk.Server) {
	if !branchIsDormant(branchSupplied, server) {
		return
	}
	env.Warn("Branch saved, but it has no effect while this server belongs to server group %q — "+
		"deployments use the group's branch (or the repository default). "+
		"Remove the server from the group, or set the branch on the group instead.",
		*server.ServerGroupIdentifier)
}

// applyToUpdate copies the supplied flags onto an update request. Flags that
// were not given stay absent from the payload — silently writing a strategy or
// a retention the operator did not ask for would clobber existing settings.
func (f *serverDeploymentFlags) applyToUpdate(req *sdk.ServerUpdateRequest) {
	if f.supplied("branch") {
		v := f.branch
		req.Branch = &v
	}
	if f.supplied("auto-deploy") {
		v := f.autoDeploy
		req.AutoDeploy = &v
	}
	if f.supplied("atomic") {
		v := f.atomic
		req.Atomic = &v
	}
	if f.supplied("atomic-strategy") {
		req.AtomicStrategy = f.atomicStrategy
	}
	if f.supplied("atomic-retention") {
		v := f.atomicRetention
		req.AtomicRetention = &v
	}
}

func newServersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "servers",
		Aliases: []string{"server", "srv"},
		Short:   "Manage servers",
		Long: `Servers are the deployment targets for a project — the destinations where your code lands. DeployHQ supports many protocols: SSH, FTP/FTPS, Rsync, S3 (and S3-compatible), DigitalOcean, Hetzner Cloud, Heroku, Netlify, Shopify, Static Hosting, and Managed VPS. Each server pins exactly one protocol.

Static Hosting and Managed VPS are managed offerings backed by DeployHQ infrastructure (beta). They require the managed-resources beta to be enabled on your account. Use "dhq launch" for guided one-command provisioning of these offerings.

Use these commands to create, configure, and list the servers attached to a project. To deploy to one, see "dhq deploy" or "dhq deployments create".`,
	}

	cmd.AddCommand(
		newServersListCmd(),
		newServersShowCmd(),
		newServersCreateCmd(),
		newServersUpdateCmd(),
		newServersDeleteCmd(),
		newServersResetHostKeyCmd(),
		newServersFromGlobalCmd(),
		newServersMetricsCmd(),
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
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(servers,
					fmt.Sprintf("%d servers", len(servers)),
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("dhq servers show <id> -p %s", projectID)},
					output.Breadcrumb{Action: "deploy", Cmd: fmt.Sprintf("dhq deploy -p %s", projectID)},
				))
			}

			if env.QuietMode {
				identifiers := make([]string, len(servers))
				for i, s := range servers {
					identifiers[i] = s.Identifier
				}
				env.WriteQuiet(identifiers)
				return nil
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
			if env.WantsJSON() {
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
	// Static Hosting (beta)
	var subdomain string
	var spaMode bool
	var subdirectory string
	// Managed VPS (beta)
	var region, size, osImage string
	// Billing guardrail (mirrors the gate in `dhq launch`)
	var acceptCost bool
	// Deployment configuration shared with `dhq servers update`
	deployFlags := &serverDeploymentFlags{}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new server",
		Example: `  # SSH server with password
  dhq servers create -p my-app --name prod --protocol-type ssh \
    --hostname prod.example.com --username deploy --password '<pw>' \
    --path /var/www/app

  # SSH server with global SSH key (recommended)
  dhq servers create -p my-app --name prod --protocol-type ssh \
    --hostname prod.example.com --username deploy --use-ssh-keys \
    --global-key-pair-id key-abc123 --path /var/www/app

  # S3 bucket
  dhq servers create -p my-app --name cdn --protocol-type s3 \
    --bucket-name assets.example.com --access-key-id AKIA... --secret-access-key '<key>'

  # Heroku app
  dhq servers create -p my-app --name staging --protocol-type heroku \
    --app-name my-app-staging --api-key '<key>'

  # Static Hosting site (beta — requires managed-resources beta)
  dhq servers create -p my-app --name site --protocol-type static_hosting \
    --subdomain my-app --subdirectory dist

  # Managed VPS droplet (beta — requires managed-resources beta)
  dhq servers create -p my-app --name vps --protocol-type managed_vps \
    --region lon1 --size s-1vcpu-1gb

  # Staging Managed VPS deploying the staging branch with atomic releases
  dhq servers create -p my-app --name staging --protocol-type managed_vps \
    --region lon1 --size s-1vcpu-1gb --accept-cost \
    --branch staging --auto-deploy --atomic --atomic-retention 5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return &output.UserError{Message: "Server name is required", Hint: "Use --name flag"}
			}
			if protocolType == "" {
				return &output.UserError{
					Message: "Protocol type is required",
					Hint:    "Use --protocol-type with one of: ssh, ftp, ftps, rsync, s3, s3_compatible, digitalocean, hetzner_cloud, heroku, netlify, shopify, static_hosting, managed_vps",
				}
			}
			// Local, offline validation — runs before a project is resolved or a
			// client is built, so a malformed invocation never touches the network.
			if err := deployFlags.validate(); err != nil {
				return err
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
				BucketName:      bucketName,
				AccessKeyID:     accessKeyID,
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
				// Managed VPS (beta)
				Region:  region,
				Size:    size,
				OSImage: osImage,
			}
			deployFlags.applyToCreate(&req)
			// Static Hosting (beta) — nested attributes
			if protocolType == "static_hosting" && subdomain != "" {
				req.HostedWebsiteAttributes = &sdk.HostedWebsiteAttributes{
					Subdomain:    subdomain,
					SPAMode:      spaMode,
					Subdirectory: subdirectory,
				}
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

			// Cost-acknowledgement guardrail: managed_vps MUST be acknowledged before
			// creation (mirrors the gate in `dhq launch --vps`) (Fix 4). The exact
			// wording is sourced from managedVPSAcknowledgePhrase() so it tracks the
			// beta-vs-GA switch in metered.go.
			if protocolType == "managed_vps" && !acceptCost {
				if !env.IsTTY || env.NonInteractive || env.JSONMode {
					return &output.UserError{
						Message: "Managed VPS creation requires --accept-cost (" + managedVPSAcknowledgePhrase() + ")",
						Hint:    "Add --accept-cost to acknowledge that a Managed VPS is " + managedVPSAcknowledgePhrase() + ".",
					}
				}
				// Interactive: prompt to confirm
				fmt.Fprintf(env.Stderr, "Creating a Managed VPS — %s. Continue? [y/N]: ", managedVPSAcknowledgePhrase()) //nolint:errcheck
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					return &output.UserError{Message: "Managed VPS creation cancelled"}
				}
			}

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

			warnIfBranchDormant(env, deployFlags.supplied("branch"), server)

			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(server, fmt.Sprintf("Created server: %s", server.Name)))
			}
			env.Status("Created server: %s (%s)", server.Name, server.Identifier)
			return nil
		},
	}

	// Common flags
	cmd.Flags().StringVar(&name, "name", "", "Server name (required)")
	cmd.Flags().StringVar(&protocolType, "protocol-type", "", "Protocol (required): ssh, ftp, ftps, rsync, s3, s3_compatible, digitalocean, hetzner_cloud, heroku, netlify, shopify, static_hosting, managed_vps")
	cmd.Flags().StringVar(&serverPath, "path", "", "Server path")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment name")

	// Deployment configuration (shared with `dhq servers update`)
	deployFlags.register(cmd)

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

	// Static Hosting (beta) — requires managed-resources beta on the account
	cmd.Flags().StringVar(&subdomain, "subdomain", "", "Globally unique subdomain under deployhq-sites.com (static_hosting)")
	cmd.Flags().BoolVar(&spaMode, "spa-mode", false, "Enable SPA routing: all paths rewrite to index.html (static_hosting)")
	cmd.Flags().StringVar(&subdirectory, "subdirectory", "", "Output subdirectory to publish, e.g. dist (static_hosting)")

	// Managed VPS (beta) — requires managed-resources beta on the account
	cmd.Flags().StringVar(&region, "region", "", "DigitalOcean region slug, e.g. lon1, nyc3 (managed_vps)")
	cmd.Flags().StringVar(&size, "size", "", "DigitalOcean droplet size slug, e.g. s-1vcpu-1gb (managed_vps)")
	cmd.Flags().StringVar(&osImage, "os-image", "", "OS image slug (managed_vps, default: ubuntu-24-04-x64)")

	// Cost-acknowledgement guardrail — required for managed_vps in non-interactive mode
	cmd.Flags().BoolVar(&acceptCost, "accept-cost", false, "Acknowledge Managed VPS provisioning — "+managedVPSAcknowledgePhrase()+" (required for non-interactive managed_vps creation)")

	return cmd
}

func newServersUpdateCmd() *cobra.Command {
	var name, serverPath, environment string
	// Deployment configuration shared with `dhq servers create`
	deployFlags := &serverDeploymentFlags{}

	cmd := &cobra.Command{
		Use:               "update <identifier>",
		Short:             "Update a server",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		Example: `  # Point a server at a different branch
  dhq servers update prod -p my-app --branch release

  # Turn atomic deployments on before the server's first deploy
  dhq servers update prod -p my-app --atomic --atomic-strategy copy_cache --atomic-retention 5

  # Turn auto-deploy off without touching any other setting
  dhq servers update prod -p my-app --auto-deploy=false`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Local, offline validation — runs before a project is resolved or a
			// client is built, so a malformed invocation never touches the network.
			if err := deployFlags.validate(); err != nil {
				return err
			}

			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			req := sdk.ServerUpdateRequest{Name: name, ServerPath: serverPath, Environment: environment}
			// Only flags the operator actually supplied reach the payload, so an
			// update never disturbs deployment settings that were left unnamed.
			deployFlags.applyToUpdate(&req)

			server, err := client.UpdateServer(cliCtx.Background(), projectID, args[0], req)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			warnIfBranchDormant(env, deployFlags.supplied("branch"), server)

			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(server, fmt.Sprintf("Updated server: %s", server.Name)))
			}
			env.Status("Updated server: %s", server.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Server name")
	cmd.Flags().StringVar(&serverPath, "path", "", "Server path")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment name")
	deployFlags.register(cmd)
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

func newServersFromGlobalCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "from-global <global-server-id>",
		Short: "Create a project server from a global server template",
		Args:  cobra.ExactArgs(1),
		Example: `  # Add a global server to the current project
  dhq servers from-global glob-abc123 -p my-app`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			server, err := client.CreateServerFromGlobal(cliCtx.Background(), projectID, args[0])
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(server,
					fmt.Sprintf("Created server: %s", server.Name),
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("dhq servers show %s -p %s", server.Identifier, projectID)},
					output.Breadcrumb{Action: "deploy", Cmd: fmt.Sprintf("dhq deploy -p %s", projectID)},
				))
			}
			env.Status("Created server: %s (%s)", server.Name, server.Identifier)
			return nil
		},
	}
}

func newServersMetricsCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "metrics <identifier>",
		Short:             "Show a point-in-time metrics snapshot for a server (beta, SSH only)",
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

			metrics, err := client.GetServerMetrics(cliCtx.Background(), projectID, args[0])
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(metrics,
					fmt.Sprintf("Metrics for server: %s", args[0]),
				))
			}

			rows := [][]string{}
			if metrics.Hostname != "" {
				rows = append(rows, []string{"Hostname", metrics.Hostname})
			}
			if online, ok := metrics.Status["online"].(bool); ok {
				reachable := "no"
				if online {
					reachable = "yes"
				}
				rows = append(rows, []string{"Reachable", reachable})
			}
			rows = append(rows,
				[]string{"Uptime", metrics.Uptime.Formatted},
				[]string{"Partitions", fmt.Sprintf("%d", len(metrics.Disk))},
			)
			env.WriteTable([]string{"Field", "Value"}, rows)
			env.Status("\nMetrics are nested; run with --json for the full status/cpu/memory/disk/profile snapshot.")
			return nil
		},
	}
}
