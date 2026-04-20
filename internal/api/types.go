package api

import "encoding/json"

// Project is a minimal projection of the /v1/projects resource.
// Fields added here match the keys the CLI renders; unused server fields
// can stay ignored.
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Active    bool   `json:"active"`
	Region    string `json:"region"`
	Runtime   string `json:"runtime"`
	Instance  int    `json:"instance"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`

	// Expand=variables fills these on GET.
	Domains   []Domain   `json:"domains,omitempty"`
	Variables []Variable `json:"variables,omitempty"`
	Rules     []Rule     `json:"rules,omitempty"`
}

// File is one entry in a remote file listing.
type File struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	IsDir      bool   `json:"is_dir"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

// Backup is one backup record for a project.
type Backup struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	Automatic bool   `json:"automatic"`
	Size      int64  `json:"size"`
}

// BackupsResponse is the shape of GET /v1/projects/{id}/backups.
type BackupsResponse struct {
	Data      []Backup `json:"data"`
	Frequency int      `json:"frequency"`
	Retention int      `json:"retention"`
}

// Region describes a runtime region.
type Region struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Runtime describes a runtime image.
type Runtime struct {
	ID           string  `json:"id"`
	Description  string  `json:"description"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	DeprecatedAt *string `json:"deprecated_at,omitempty"`
}

// Variable is one env var attached to a project.
type Variable struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Sensitive bool    `json:"sensitive"`
	Value     *string `json:"value"`
}

// Domain is a custom domain attached to a project.
type Domain struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Rule is an untyped rule body. The structure varies and the CLI only
// renders it as JSON, so we keep the raw form.
type Rule = json.RawMessage

// UsageRecord is one day of project usage stats.
type UsageRecord struct {
	EventsAt        string  `json:"events_at"`
	RequestsTotal   int64   `json:"requests_total"`
	Responses2xx    int64   `json:"responses_2xx"`
	Responses4xx    int64   `json:"responses_4xx"`
	Responses5xx    int64   `json:"responses_5xx"`
	DurationAverage float64 `json:"duration_average"`
	DatatransferOut float64 `json:"datatransfer_out"`
}

// LogEntry is one log event from /v1/projects/{id}/logs.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Priority  string `json:"priority"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// Status is the /status endpoint response.
type Status struct {
	Status string `json:"status"`
}
