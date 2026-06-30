// Package sdk provides a Go client for the DeployHQ API.
//
// The SDK is designed as a clean public interface boundary that can be
// extracted into a standalone module (deployhq/deployhq-go) in the future.
package sdk

import (
	"encoding/json"
	"fmt"
	"time"
)

// FlexString handles API fields that may be returned as either a string or a number.
// The DeployHQ API is inconsistent with some fields (e.g., Server.Port).
type FlexString string

func (f *FlexString) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexString(s)
		return nil
	}
	// Try number
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexString(n.String())
		return nil
	}
	return fmt.Errorf("FlexString: cannot unmarshal %s", string(data))
}

func (f FlexString) String() string { return string(f) }

// Project represents a DeployHQ project.
type Project struct {
	Name           string      `json:"name"`
	Permalink      string      `json:"permalink"`
	Identifier     string      `json:"identifier"`
	PublicKey      string      `json:"public_key"`
	Repository     *Repository `json:"repository,omitempty"`
	RepositoryURL  *string     `json:"repository_url,omitempty"`
	Zone           string      `json:"zone"`
	LastDeployedAt *string     `json:"last_deployed_at,omitempty"`
	AutoDeployURL  string      `json:"auto_deploy_url"`
	Starred        bool        `json:"starred,omitempty"`
}

// ProjectCreateRequest is the payload for creating a project.
type ProjectCreateRequest struct {
	Name              string `json:"name"`
	KeypairIdentifier string `json:"keypair_identifier,omitempty"`
	ZoneID            string `json:"zone_id,omitempty"`
	TemplateID        string `json:"template_id,omitempty"`
}

// ProjectUpdateRequest is the payload for updating a project.
type ProjectUpdateRequest struct {
	Name                   string `json:"name,omitempty"`
	Permalink              string `json:"permalink,omitempty"`
	ZoneID                 string `json:"zone_id,omitempty"`
	EmailNotifyOn          string `json:"email_notify_on,omitempty"`
	NotificationEmail      string `json:"notification_email,omitempty"`
	NotifyPusher           *bool  `json:"notify_pusher,omitempty"`
	CustomPrivateKey       string `json:"custom_private_key,omitempty"`
	CheckUndeployedChanges string `json:"check_undeployed_changes,omitempty"`
	StoreArtifactsEnabled  string `json:"store_artifacts_enabled,omitempty"`
}

// Server represents a deployment server.
type Server struct {
	ID                     int          `json:"id"`
	Identifier             string       `json:"identifier"`
	Name                   string       `json:"name"`
	ProtocolType           string       `json:"protocol_type"`
	ServerPath             string       `json:"server_path"`
	LastRevision           string       `json:"last_revision"`
	PreferredBranch        string       `json:"preferred_branch"`
	Branch                 string       `json:"branch"`
	NotifyEmail            string       `json:"notify_email"`
	ServerGroupIdentifier  *string      `json:"server_group_identifier,omitempty"`
	AutoDeploy             bool         `json:"auto_deploy"`
	Environment            string       `json:"environment"`
	Enabled                bool         `json:"enabled"`
	Agent                  *ServerAgent `json:"agent,omitempty"`
	Atomic                 *bool        `json:"atomic,omitempty"`
	AtomicStrategy         string       `json:"atomic_strategy"`
	AtomicRetention        int          `json:"atomic_retention"`
	UseCompression         bool         `json:"use_compression"`
	UseAcceleratedTransfer bool         `json:"use_accelerated_transfer"`
	UseParallelUpload      bool         `json:"use_parallel_upload"`
	RootPath               string       `json:"root_path"`
	Position               int          `json:"position"`
	CreatedAt              string       `json:"created_at"`
	UpdatedAt              string       `json:"updated_at"`
	ConnectionCheckedAt    *string      `json:"connection_checked_at,omitempty"`
	ConnectionErrorMessage *string      `json:"connection_error_message,omitempty"`
	Hostname               string       `json:"hostname,omitempty"`
	Username               string       `json:"username,omitempty"`
	Port                   FlexString   `json:"port,omitempty"`
	UseSSHKeys             bool         `json:"use_ssh_keys,omitempty"`
	HostKey                string       `json:"host_key,omitempty"`
	UnlinkBeforeUpload     bool         `json:"unlink_before_upload,omitempty"`

	// Managed-resource provisioning state, populated for managed_vps and
	// static_hosting servers by the project-scoped server show endpoint
	// (GET /projects/:id/servers/:id). The backend nests provisioning
	// detail under a protocol-specific block; use ProvisioningStatus(server)
	// to read the lifecycle status uniformly across protocols.
	StaticHosting *StaticHostingInfo `json:"static_hosting,omitempty"`
	ManagedVPS    *ManagedVPSInfo    `json:"managed_vps,omitempty"`
}

// HostedWebsiteAttributes holds Static Hosting-specific provisioning parameters.
// Set this when creating a server with protocol_type "static_hosting".
type HostedWebsiteAttributes struct {
	// Subdomain is the globally unique subdomain under deployhq-sites.com.
	// Example: "my-app" → https://my-app.deployhq-sites.com
	Subdomain string `json:"subdomain"`
	// SPAMode enables single-page-application routing (rewrites all paths to index.html).
	SPAMode bool `json:"spa_mode,omitempty"`
	// Subdirectory is the output folder within the server path to publish.
	// Defaults to "" (the server path root) when empty.
	Subdirectory string `json:"subdirectory,omitempty"`
}

// ServerCreateRequest is the payload for creating a server.
// Fields are a flat union of all protocol types; the API ignores
// fields that don't apply to the chosen protocol_type.
type ServerCreateRequest struct {
	// Common
	Name         string `json:"name"`
	ProtocolType string `json:"protocol_type"`
	ServerPath   string `json:"server_path,omitempty"`
	Environment  string `json:"environment,omitempty"`
	RootPath     string `json:"root_path,omitempty"`
	AgentID      string `json:"agent_id,omitempty"`
	Enabled      *bool  `json:"enabled,omitempty"`

	// SSH / FTP / FTPS / Rsync
	Hostname        string `json:"hostname,omitempty"`
	Port            *int   `json:"port,omitempty"`
	Username        string `json:"username,omitempty"`
	Password        string `json:"password,omitempty"`
	UseSSHKeys      *bool  `json:"use_ssh_keys,omitempty"`
	GlobalKeyPairID string `json:"global_key_pair_id,omitempty"`

	// S3
	BucketName      string `json:"bucket_name,omitempty"`
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`

	// S3-Compatible
	CustomEndpoint string `json:"custom_endpoint,omitempty"`

	// DigitalOcean
	PersonalAccessToken string `json:"personal_access_token,omitempty"`
	DropletName         string `json:"droplet_name,omitempty"`

	// Hetzner Cloud
	APIToken          string `json:"api_token,omitempty"`
	HetznerServerName string `json:"hetzner_server_name,omitempty"`

	// Heroku
	AppName      string `json:"app_name,omitempty"`
	APIKeyHeroku string `json:"api_key_heroku,omitempty"`

	// Netlify
	SiteID      string `json:"site_id,omitempty"`
	AccessToken string `json:"access_token,omitempty"`

	// Shopify
	StoreURL  string `json:"store_url,omitempty"`
	ThemeName string `json:"theme_name,omitempty"`

	// Managed-resource provisioning params. These are tagged `json:"-"` because
	// the DeployHQ servers API expects them as TOP-LEVEL siblings of `server` in
	// the request body (params[:region], params[:hosted_website_attributes], …),
	// NOT nested inside the server object. CreateServer hoists them accordingly.

	// Static Hosting (protocol_type "static_hosting") — attributes that configure
	// the HostedWebsite provisioned alongside the server.
	HostedWebsiteAttributes *HostedWebsiteAttributes `json:"-"`
	// Region is the DigitalOcean region slug for a Managed VPS (e.g. "lon1", "nyc3").
	Region string `json:"-"`
	// Size is the DigitalOcean droplet size slug (e.g. "s-1vcpu-1gb").
	Size string `json:"-"`
	// OSImage is the DigitalOcean image slug (defaults to "ubuntu-24-04-x64" when empty).
	OSImage string `json:"-"`
}

// ServerUpdateRequest is the payload for updating a server.
type ServerUpdateRequest struct {
	Name         string `json:"name,omitempty"`
	ProtocolType string `json:"protocol_type,omitempty"`
	ServerPath   string `json:"server_path,omitempty"`
	Environment  string `json:"environment,omitempty"`
	RootPath     string `json:"root_path,omitempty"`
	Enabled      *bool  `json:"enabled,omitempty"`
}

// ServerGroup represents a group of servers.
type ServerGroup struct {
	Identifier      string   `json:"identifier"`
	Name            string   `json:"name"`
	Servers         []Server `json:"servers"`
	PreferredBranch string   `json:"preferred_branch"`
	LastRevision    string   `json:"last_revision"`
	Environment     string   `json:"environment"`
}

// ServerGroupCreateRequest is the payload for creating a server group.
type ServerGroupCreateRequest struct {
	Name string `json:"name"`
}

// ServerGroupUpdateRequest is the payload for updating a server group.
type ServerGroupUpdateRequest struct {
	Name string `json:"name,omitempty"`
}

// ServerAgent is the embedded agent object within a server response.
// The API returns the full agent object, not just an identifier.
type ServerAgent struct {
	ID         int     `json:"id"`
	CreatedAt  string  `json:"created_at"`
	Identifier string  `json:"identifier"`
	Name       string  `json:"name"`
	Online     bool    `json:"online"`
	RevokedAt  *string `json:"revoked_at,omitempty"`
	UpdatedAt  string  `json:"updated_at"`
}

// Deployment represents a deployment.
type Deployment struct {
	Identifier            string             `json:"identifier"`
	Servers               []Server           `json:"servers"`
	Project               *DeploymentProject `json:"project,omitempty"`
	Deployer              *string            `json:"deployer,omitempty"`
	DeployerAvatar        *string            `json:"deployer_avatar,omitempty"`
	Branch                string             `json:"branch"`
	StartRevision         *Revision          `json:"start_revision,omitempty"`
	EndRevision           *Revision          `json:"end_revision,omitempty"`
	Status                string             `json:"status"`
	Timestamps            *Timestamps        `json:"timestamps,omitempty"`
	Configuration         *DeployConfig      `json:"configuration,omitempty"`
	Legacy                bool               `json:"legacy"`
	Deferred              bool               `json:"deferred"`
	ConfigFilesDeployment bool               `json:"config_files_deployment"`
	Overview              *string            `json:"overview,omitempty"`
	Archived              bool               `json:"archived"`
	ArchivedAt            *string            `json:"archived_at,omitempty"`
	LogSummary            *string            `json:"log_summary,omitempty"`
	Steps                 []DeploymentStep   `json:"steps,omitempty"`
}

// DeploymentProject is the embedded project info within a deployment response.
type DeploymentProject struct {
	Name           string      `json:"name"`
	Permalink      string      `json:"permalink"`
	Identifier     string      `json:"identifier"`
	PublicKey      string      `json:"public_key"`
	Repository     *Repository `json:"repository,omitempty"`
	RepositoryURL  string      `json:"repository_url"`
	Zone           string      `json:"zone"`
	LastDeployedAt string      `json:"last_deployed_at"`
	AutoDeployURL  string      `json:"auto_deploy_url"`
}

// Revision holds a git revision reference.
type Revision struct {
	Ref string `json:"ref"`
}

// Timestamps holds deployment timing information.
type Timestamps struct {
	QueuedAt    string      `json:"queued_at"`
	StartedAt   *string     `json:"started_at,omitempty"`
	CompletedAt *string     `json:"completed_at,omitempty"`
	Duration    *FlexString `json:"duration,omitempty"`
	RunsAt      string      `json:"runs_at,omitempty"`
}

// DeployConfig holds deployment configuration options.
type DeployConfig struct {
	CopyConfigFiles       bool   `json:"copy_config_files"`
	NotificationAddresses string `json:"notification_addresses"`
	SkipProjectFiles      bool   `json:"skip_project_files"`
}

// DeploymentStep represents a step within a deployment.
type DeploymentStep struct {
	Step                string      `json:"step"`
	Stage               string      `json:"stage"`
	Identifier          string      `json:"identifier"`
	Server              *FlexString `json:"server,omitempty"`
	TotalItems          *FlexString `json:"total_items,omitempty"`
	CompletedItems      *FlexString `json:"completed_items,omitempty"`
	Description         string      `json:"description"`
	Status              string      `json:"status"`
	Logs                bool        `json:"logs"`
	DeploymentStartedAt *string     `json:"deployment_started_at,omitempty"`
	UpdatedAt           string      `json:"updated_at"`
}

// DeploymentCreateRequest is the payload for creating a deployment.
type DeploymentCreateRequest struct {
	StartRevision         string `json:"start_revision,omitempty"`
	EndRevision           string `json:"end_revision,omitempty"`
	CopyConfigFiles       *bool  `json:"copy_config_files,omitempty"`
	NotificationAddresses string `json:"notification_addresses,omitempty"`
	Branch                string `json:"branch,omitempty"`
	ParentIdentifier      string `json:"parent_identifier,omitempty"`
	ServerIdentifier      string `json:"server_identifier,omitempty"`
	RunBuildCommands      *bool  `json:"run_build_commands,omitempty"`
	UseBuildCache         *bool  `json:"use_build_cache,omitempty"`
	ConfigFilesDeployment *bool  `json:"config_files_deployment,omitempty"`
	Mode                  string `json:"mode,omitempty"`
	UseLatest             *bool  `json:"use_latest,omitempty"`
}

// DeploymentPreview is the minimal response from a preview deployment.
// The API returns only status and identifier for preview mode.
type DeploymentPreview struct {
	Status     string `json:"status"`
	Identifier string `json:"identifier"`
}

// Repository represents a project's repository configuration.
type Repository struct {
	ScmType        string          `json:"scm_type"`
	URL            string          `json:"url"`
	Port           *int            `json:"port,omitempty"`
	Username       *string         `json:"username,omitempty"`
	Branch         string          `json:"branch"`
	Cached         bool            `json:"cached"`
	HostingService *HostingService `json:"hosting_service,omitempty"`
}

// HostingService contains hosting provider metadata.
type HostingService struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	TreeURL    string `json:"tree_url"`
	CommitsURL string `json:"commits_url"`
}

// RepositoryCreateRequest is the payload for creating/updating a repository.
type RepositoryCreateRequest struct {
	ScmType  string `json:"scm_type"`
	URL      string `json:"url"`
	Port     *int   `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Branch   string `json:"branch,omitempty"`
}

// Commit represents a repository commit.
type Commit struct {
	Ref          string    `json:"ref"`
	Author       string    `json:"author"`
	Email        string    `json:"email"`
	Timestamp    time.Time `json:"timestamp"`
	Message      string    `json:"message"`
	ShortMessage string    `json:"short_message"`
	Tags         []string  `json:"tags,omitempty"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
}

// CommitsTagsReleases is the response for recent_commits containing
// commits, tags, and releases.
type CommitsTagsReleases struct {
	Commits  []Commit `json:"commits"`
	Tags     []string `json:"tags"`
	Releases []string `json:"releases"`
}

// DeploymentStepLog represents logs for a deployment step.
type DeploymentStepLog struct {
	ID      FlexString `json:"id"`
	Step    string     `json:"step"`
	Type    string     `json:"type,omitempty"`
	Detail  *string    `json:"detail,omitempty"`
	Message string     `json:"message"`
}

// Pagination holds pagination metadata from list responses.
type Pagination struct {
	CurrentPage  int `json:"current_page"`
	TotalPages   int `json:"total_pages"`
	TotalRecords int `json:"total_records"`
	Offset       int `json:"offset"`
}

// PaginatedResponse wraps a list response with pagination info.
type PaginatedResponse[T any] struct {
	Pagination Pagination `json:"pagination"`
	Records    []T        `json:"records"`
}

// ListOptions provides optional pagination parameters for list endpoints.
type ListOptions struct {
	Page    int
	PerPage int
}

// AccountCapabilities holds the managed-resource capability flags carried on the
// account sub-object of GET /profile.
// All authenticated account members can read /profile — it is not admin-gated.
type AccountCapabilities struct {
	// BetaFeatures indicates whether the managed-resources beta is enabled for this account.
	BetaFeatures bool `json:"beta_features"`
	// StaticHostingEligible indicates whether the account can provision Static Hosting sites.
	StaticHostingEligible bool `json:"static_hosting_eligible"`
	// ManagedVPSEligible indicates whether the account can provision Managed VPS droplets.
	ManagedVPSEligible bool `json:"managed_vps_eligible"`
}

// BetaEnrollmentRequest is the body for POST /beta/enrollments.
// Protocol is optional — omit to enroll in all managed-resources protocols.
type BetaEnrollmentRequest struct {
	// Protocol is the managed protocol to enroll in, e.g. "static_hosting" or "managed_vps".
	// When empty the backend enrolls the account in all managed-resources protocols.
	Protocol string `json:"protocol,omitempty"`
}

// BetaEnrollmentResponse is the response from POST /beta/enrollments.
type BetaEnrollmentResponse struct {
	// Enrolled is true when the account is now enrolled (was already enrolled or just flipped).
	Enrolled bool `json:"enrolled"`
	// BetaFeatures mirrors the account's beta_features flag after the operation.
	BetaFeatures bool `json:"beta_features"`
}

// ManagedHostingRegion is a DigitalOcean region available for Managed VPS provisioning.
type ManagedHostingRegion struct {
	// Slug is the region identifier used in ServerCreateRequest.Region (e.g. "lon1").
	Slug string `json:"slug"`
	// Name is the human-readable region name (e.g. "London, United Kingdom").
	Name string `json:"name"`
	// Available indicates whether new droplets can currently be created in this region.
	Available bool `json:"available"`
}

// ManagedHostingSize is a DigitalOcean droplet size available for Managed VPS provisioning.
type ManagedHostingSize struct {
	// Slug is the size identifier used in ServerCreateRequest.Size (e.g. "s-1vcpu-1gb").
	Slug string `json:"slug"`
	// Description is the human-readable size label (e.g. "1 vCPU / 1 GB RAM").
	Description string `json:"description"`
	// PriceMonthly is the monthly cost in the account's billing currency.
	PriceMonthly float64 `json:"price_monthly"`
	// PriceHourly is the hourly cost in the account's billing currency.
	PriceHourly float64 `json:"price_hourly"`
	// Memory is the RAM in megabytes.
	Memory int `json:"memory"`
	// VCPUs is the number of virtual CPUs.
	VCPUs int `json:"vcpus"`
	// Disk is the root disk size in gigabytes.
	Disk int `json:"disk"`
}

// ManagedVPSInfo is the nested `managed_vps` object within a server response for
// managed_vps servers. It carries the provisioning state and droplet
// detail that live on the backing HostedResource rather than on the server row.
type ManagedVPSInfo struct {
	// HostedResourceIdentifier is the opaque identifier of the backing HostedResource.
	HostedResourceIdentifier string `json:"hosted_resource_identifier"`
	// Status is the provisioning lifecycle status, e.g. "provisioning", "active", "error".
	Status string `json:"status,omitempty"`
	// IPAddress is the public IP of the droplet once active.
	IPAddress string `json:"ip_address,omitempty"`
	// Region is the DigitalOcean region slug the droplet runs in.
	Region string `json:"region,omitempty"`
	// Size is the DigitalOcean droplet size slug.
	Size string `json:"size,omitempty"`
	// MonthlyCost is the droplet's monthly cost (string or number on the wire).
	MonthlyCost FlexString `json:"monthly_cost,omitempty"`
}

// StaticHostingInfo is the nested `static_hosting` object within a server response for static_hosting servers.
type StaticHostingInfo struct {
	// URL is the live public URL of the Static Hosting site (e.g. "https://my-app.deployhq-sites.com").
	URL string `json:"url"`
	// Subdomain is the subdomain portion of the URL.
	Subdomain string `json:"subdomain"`
	// Status is the provisioning status of the underlying HostedWebsite.
	Status string `json:"status,omitempty"`
}

// TwoFactorError is a sentinel returned by Signup when the API responds with 422
// and the error body indicates an existing account with 2FA enabled.
// The caller should redirect the user to browser-based login.
type TwoFactorError struct {
	// Message is the human-readable error from the API.
	Message string
}

func (e *TwoFactorError) Error() string {
	return "two-factor authentication required: " + e.Message
}
