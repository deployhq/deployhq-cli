package sdk

import "context"

// ActivityEvent represents a single activity event.
type ActivityEvent struct {
	Event      string                 `json:"event"`
	Project    ActivityProject        `json:"project"`
	Properties map[string]interface{} `json:"properties"`
	User       string                 `json:"user"`
	CreatedAt  string                 `json:"created_at"`
}

// ActivityProject is the project reference inside an activity event.
type ActivityProject struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Permalink  string `json:"permalink"`
}

// ActivityStats contains deploy stats for the current week.
type ActivityStats struct {
	DeploymentsThisWeek int     `json:"deployments_this_week"`
	DeploymentsDelta    int     `json:"deployments_delta"`
	SuccessRate         float64 `json:"success_rate"`
	SuccessRateDelta    float64 `json:"success_rate_delta"`
	AvgDurationSeconds  int     `json:"avg_duration_seconds"`
	ActiveServers       int     `json:"active_servers"`
}

// ActivityWithStats wraps events and stats from GET /activity?include=stats.
type ActivityWithStats struct {
	Events []ActivityEvent `json:"events"`
	Stats  ActivityStats   `json:"stats"`
}

// ListActivity returns recent activity events for the account.
func (c *Client) ListActivity(ctx context.Context, opts *ListOptions) ([]ActivityEvent, error) {
	var result []ActivityEvent
	path := appendListParams("/activity", opts)
	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListActivityWithStats returns recent activity events and deploy stats.
func (c *Client) ListActivityWithStats(ctx context.Context, opts *ListOptions) (*ActivityWithStats, error) {
	var result ActivityWithStats
	path := appendListParams("/activity?include=stats", opts)
	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
