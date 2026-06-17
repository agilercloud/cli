package api

import "github.com/agilercloud/cli/internal/publicapi"

// Type aliases that re-export generated wire shapes under the api
// package's namespace. CLI command code spells these as api.Project
// rather than publicapi.ProjectSummary so the api package presents one
// cohesive surface and the CLI files import only one package for both
// types and operations.
//
// New types are added by updating the OpenAPI spec, regenerating the
// publicapi client, and adding a line below — never by redefining the
// shape here.

type (
	Project       = publicapi.ProjectSummary
	ProjectDetail = publicapi.ProjectDetail

	Domain   = publicapi.ProjectDomainOutput
	Variable = publicapi.ProjectVariableOutput
	Backup   = publicapi.ProjectBackupEntry
	File     = publicapi.ProjectFileEntry
	Rule     = publicapi.ProjectRuleOutput
	LogEntry = publicapi.ProjectLogEventOutput
	Usage    = publicapi.ProjectUsageOutput

	BackupPolicy = publicapi.ProjectBackupPolicyOutput
	RuleOptions  = publicapi.RuleOptionsOutput
	Region       = publicapi.RegionOutput
	Runtime      = publicapi.RuntimeOutput
	Workspace    = publicapi.WorkspaceOutput

	WorkspaceMember          = publicapi.WorkspaceMemberOutput
	WorkspaceBillingTransfer = publicapi.WorkspaceBillingTransferOutput

	SelfUser     = publicapi.SelfUser
	Billing      = publicapi.BillingOutput
	BillingMonth = publicapi.BillingTransactionMonth
	BillingTx    = publicapi.BillingTransactionEntry
	Notification = publicapi.NotificationOutput

	CreateProjectInput   = publicapi.CreateProjectInput
	CreateWorkspaceInput = publicapi.CreateWorkspaceInput
	UpdateProjectInput   = publicapi.UpdateProjectInput
	CreateDomainInput    = publicapi.CreateProjectDomainInput
	UpdateDomainInput    = publicapi.UpdateProjectDomainInput
	CreateVariableInput  = publicapi.CreateProjectVariableInput
	UpdateVariableInput  = publicapi.UpdateProjectVariableInput
	CreateRuleInput      = publicapi.CreateProjectRuleInput
	UpdateRuleInput      = publicapi.UpdateProjectRuleInput
	UpdateBackupPolicy   = publicapi.UpdateProjectBackupPolicyInput
	UpdateBillingInput   = publicapi.UpdateBillingInput
	CreateSQLStatement   = publicapi.CreateSQLStatementInput
	CreateWPCommand      = publicapi.CreateWPCommandInput
	UsageGranularity     = publicapi.GetProjectUsageParamsGranularity

	// SQLStatementListItem is the slim per-entry shape returned by the
	// /sql/statements list endpoint (id, status, submitted_at, duration_ms,
	// sql_preview). The full entity — including the unredacted SQL, columns,
	// rows, etc. — is only returned by the single-statement GET, which
	// keeps using the hand-rolled SQLStatement type below.
	SQLStatementListItem = publicapi.StatementListItem

	// WPCommandListItem is the slim per-entry shape returned by the
	// /wp/commands list endpoint (id, status, submitted_at, duration_ms,
	// command_preview). The full entity — including the untruncated
	// command, exit code, output lines, etc. — is only returned by the
	// single-command GET, which keeps using the hand-rolled WPCommand
	// type below.
	WPCommandListItem = publicapi.WPCommandListItem
)

// Granularity constants for project usage queries. Mirror publicapi's
// generated values; re-exported so CLI commands don't import publicapi.
const (
	GranularityHour  = publicapi.Hour
	GranularityDay   = publicapi.Day
	GranularityWeek  = publicapi.Week
	GranularityMonth = publicapi.Month
)

// SQLStatement is the CLI's view of a SQL execution row. The public spec
// types the SQL endpoints as a freeform object (the shape varies by
// status and carries many optional fields), so we keep an explicit
// struct here to get typed rendering.
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

	// NextCursor pages the remaining result rows when Rows was truncated.
	// Parsed from the response's Link rel="next" header, not the body;
	// empty when this page is the last.
	NextCursor string `json:"next_cursor,omitempty"`
}

// WPCommand is the CLI's view of a wp-cli command execution. The public
// spec types the WP endpoints as a freeform object (the shape varies by
// status and carries many optional fields), so we keep an explicit
// struct here to get typed rendering.
type WPCommand struct {
	ID          string   `json:"id"`
	Command     string   `json:"command"`
	Timeout     *int     `json:"timeout,omitempty"`
	Status      string   `json:"status"`
	SubmittedAt string   `json:"submitted_at"`
	SubmittedBy string   `json:"submitted_by,omitempty"`
	CompletedAt string   `json:"completed_at,omitempty"`
	DurationMs  *int64   `json:"duration_ms"`
	ExitCode    *int     `json:"exit_code"`
	LineCount   *int64   `json:"line_count"`
	Output      []string `json:"output"`
	Error       *string  `json:"error"`

	// NextCursor pages the remaining output lines when Output was
	// truncated. Parsed from the response's Link rel="next" header, not
	// the body; empty when this page is the last.
	NextCursor string `json:"next_cursor,omitempty"`
}

// Status is the /status health-probe response. The endpoint is outside
// the v1 spec, so the type is hand-rolled.
type Status struct {
	Status string `json:"status"`
}
