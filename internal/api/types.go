package api

import "encoding/json"

// Project is the per-project response shape for /v1/projects and
// /v1/projects/{id}. Sub-resources (domains, variables, rules, backups)
// live behind their own endpoints; they are not embedded here.
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Active    bool   `json:"active"`
	Region    string `json:"region"`
	Runtime   string `json:"runtime"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
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

// BackupPolicy is the shape of GET /v1/projects/{id}/backups/policy.
type BackupPolicy struct {
	FrequencyDays int `json:"frequency_days"`
	RetentionDays int `json:"retention_days"`
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

// Domain is a custom domain attached to a project. Primary marks which
// domain serves as the project's site URL.
type Domain struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Primary bool   `json:"primary"`
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

// SelfUser is the response from GET /v1/users/me. EffectiveScopes lists
// the permission scopes the caller's credentials currently grant (with
// implication rules already resolved server-side, minus the synthetic
// session_only scope).
type SelfUser struct {
	ID              string   `json:"id"`
	CreatedAt       string   `json:"created_at"`
	Email           string   `json:"email"`
	Name            string   `json:"name"`
	SiteAdmin       bool     `json:"site_admin,omitempty"`
	EffectiveScopes []string `json:"effective_scopes"`
}

// Billing is the response from GET /v1/users/me/billing.
type Billing struct {
	Balance        float64 `json:"balance"`
	Currency       string  `json:"currency"`
	Plan           string  `json:"plan,omitempty"`
	PaymentMethod  string  `json:"payment_method,omitempty"`
	AutoPay        bool    `json:"auto_pay"`
	UpdateRequired bool    `json:"update_required"`
}

// BillingTransaction is one row in GET /v1/users/me/billing/transactions.
type BillingTransaction struct {
	ID          string  `json:"id"`
	CreatedAt   string  `json:"created_at"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Kind        string  `json:"kind,omitempty"`
}

// Notification is one row in GET /v1/users/me/notifications.
type Notification struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	ReadAt    string `json:"read_at,omitempty"`
	Subject   string `json:"subject"`
	Body      string `json:"body,omitempty"`
}

// SQLStatement is the shape returned by the SQL execution endpoints:
// POST /v1/projects/{id}/sql/statements, GET /v1/projects/{id}/sql/statements,
// and GET /v1/projects/{id}/sql/statements/{stmtId}. The Rows field is the
// page of result rows for the current request; RowCount reports the total
// across all pages. Rows use `[]any` for the inner cells so duplicate
// column names and explicit column ordering survive the wire.
//
// Status is one of pending / success / error / cancelled. SubmittedBy
// records the uid of the user that submitted the statement (empty for
// machine submissions).
type SQLStatement struct {
	ID           string   `json:"id"`
	SQL          string   `json:"sql"`
	ReadOnly     bool     `json:"read_only"`
	Timeout      *int     `json:"timeout,omitempty"`
	Status       string   `json:"status"`
	SubmittedAt  string   `json:"submitted_at"`
	SubmittedBy  string   `json:"submitted_by,omitempty"`
	CompletedAt  string   `json:"completed_at,omitempty"`
	DurationMs   *int64   `json:"duration_ms"`
	RowCount     *int64   `json:"row_count"`
	RowsAffected *int64   `json:"rows_affected"`
	Columns      []string `json:"columns"`
	Rows         [][]any  `json:"rows"`
	Error        *string  `json:"error"`
}
