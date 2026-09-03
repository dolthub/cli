package dolthub

import (
	"encoding/json"
	"time"
)

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

type DatabaseRef struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

type Database struct {
	Owner            string       `json:"owner"`
	Name             string       `json:"name"`
	Description      string       `json:"description,omitempty"`
	Visibility       Visibility   `json:"visibility"`
	ForkNetworkCount int          `json:"fork_network_count"`
	StarCount        int          `json:"star_count"`
	SizeBytes        int64        `json:"size_bytes"`
	LastWriteAt      *time.Time   `json:"last_write_at,omitempty"`
	Parent           *DatabaseRef `json:"parent,omitempty"`
	NetworkRoot      *DatabaseRef `json:"network_root,omitempty"`
}

type CreateDatabaseRequest struct {
	Owner       string     `json:"owner"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Visibility  Visibility `json:"visibility"`
}

type Branch struct {
	Name          string     `json:"name"`
	HeadCommitSHA string     `json:"head_commit_sha"`
	LastUpdatedAt *time.Time `json:"last_updated_at,omitempty"`
}

// RevisionSource is the CreateBranchRequest/CreateTagRequest discriminated
// union. Exactly one of Branch or Commit is set by command validation.
type RevisionSource struct {
	Branch string `json:"branch,omitempty"`
	Commit string `json:"commit,omitempty"`
}

type CreateBranchRequest struct {
	Name string         `json:"name"`
	From RevisionSource `json:"from"`
}

type Tag struct {
	Name      string     `json:"name"`
	CommitSHA string     `json:"commit_sha"`
	Message   string     `json:"message,omitempty"`
	TaggedAt  *time.Time `json:"tagged_at,omitempty"`
}

type CreateTagRequest struct {
	Name    string         `json:"name"`
	From    RevisionSource `json:"from"`
	Message *string        `json:"message,omitempty"`
}

type Release struct {
	Tag         string    `json:"tag"`
	Title       string    `json:"title"`
	CommitSHA   string    `json:"commit_sha"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateReleaseRequest struct {
	Tag                  string  `json:"tag"`
	Title                string  `json:"title"`
	CommitSHA            string  `json:"commit_sha"`
	Description          *string `json:"description,omitempty"`
	CreateTagIfNotExists bool    `json:"create_tag_if_not_exists,omitempty"`
}

type PullState string

const (
	PullStateOpen   PullState = "open"
	PullStateClosed PullState = "closed"
	PullStateMerged PullState = "merged"
)

type BranchRef struct {
	Database   DatabaseRef `json:"database"`
	BranchName string      `json:"branch_name"`
}

type PullSummary struct {
	PullNumber  int64     `json:"pull_number"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	State       PullState `json:"state"`
	CreatedAt   time.Time `json:"created_at"`
	Creator     string    `json:"creator"`
}

type Pull struct {
	PullNumber  int64     `json:"pull_number"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	State       PullState `json:"state"`
	FromBranch  BranchRef `json:"from_branch"`
	ToBranch    BranchRef `json:"to_branch"`
	CreatedAt   time.Time `json:"created_at"`
	Creator     string    `json:"creator"`
}

type PullComment struct {
	CommentID string    `json:"comment_id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreatePullRequest struct {
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	FromBranch  BranchRef `json:"from_branch"`
	ToBranch    BranchRef `json:"to_branch"`
}

type CreatePullCommentRequest struct {
	Body string `json:"body"`
}

type MergePullRequest struct{}

type UpdatePullRequest struct {
	Title       *string    `json:"title,omitempty"`
	Description *string    `json:"description,omitempty"`
	State       *PullState `json:"state,omitempty"`
}

type OperationStatus string

const (
	OperationQueued    OperationStatus = "queued"
	OperationRunning   OperationStatus = "running"
	OperationSucceeded OperationStatus = "succeeded"
	OperationFailed    OperationStatus = "failed"
)

type OperationType string

const (
	OperationImport   OperationType = "import"
	OperationMerge    OperationType = "merge"
	OperationSQLWrite OperationType = "sql_write"
	OperationFork     OperationType = "fork"
	OperationDoltCI   OperationType = "dolt_ci"
)

type OperationError struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
}

type Operation struct {
	ID         string          `json:"id"`
	Type       OperationType   `json:"type"`
	Status     OperationStatus `json:"status"`
	CreatedAt  time.Time       `json:"created_at"`
	Cancelable bool            `json:"cancelable"`
	Error      *OperationError `json:"error,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
}

type OperationRef struct {
	ID   string `json:"id"`
	Href string `json:"href"`
}

type CreateForkRequest struct {
	Owner string `json:"owner"`
}

type ImportFileType string

const (
	ImportCSV  ImportFileType = "csv"
	ImportPSV  ImportFileType = "psv"
	ImportXLSX ImportFileType = "xlsx"
	ImportJSON ImportFileType = "json"
	ImportSQL  ImportFileType = "sql"
	ImportYAML ImportFileType = "yaml"
)

type ImportOperation string

const (
	ImportCreate    ImportOperation = "create"
	ImportOverwrite ImportOperation = "overwrite"
	ImportUpdate    ImportOperation = "update"
	ImportReplace   ImportOperation = "replace"
)

type CompletedPart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}

type ImportUploadPart struct {
	PartNumber int    `json:"part_number"`
	URL        string `json:"url"`
}

type ImportUpload struct {
	Token       string              `json:"token"`
	ContentsKey string              `json:"contents_key"`
	Parts       []ImportUploadPart  `json:"parts"`
	HTTPMethod  string              `json:"http_method"`
	Headers     map[string][]string `json:"headers"`
}

type CreateImportUploadRequest struct {
	ContentLength int64          `json:"content_length"`
	NumParts      int            `json:"num_parts"`
	FileType      ImportFileType `json:"file_type"`
}

type CreateImportRequest struct {
	BranchName            string            `json:"branch_name"`
	TableName             string            `json:"table_name"`
	FileName              string            `json:"file_name"`
	FileSize              int64             `json:"file_size"`
	FileType              ImportFileType    `json:"file_type"`
	ImportOperation       ImportOperation   `json:"import_operation"`
	Token                 string            `json:"token"`
	ContentsKey           string            `json:"contents_key"`
	CompletedParts        []CompletedPart   `json:"completed_parts"`
	FilePartsMD5          string            `json:"file_parts_md5"`
	PrimaryKeys           []string          `json:"primary_keys"`
	CommitMessage         *string           `json:"commit_message,omitempty"`
	PullRequestBranchName *string           `json:"pull_request_branch_name,omitempty"`
	ColumnMap             map[string]string `json:"column_map,omitempty"`
}

type QueryColumn struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	IsPrimaryKey bool   `json:"is_primary_key,omitempty"`
	SourceTable  string `json:"source_table,omitempty"`
}

type QueryStatus string

const (
	QuerySuccess      QueryStatus = "success"
	QueryError        QueryStatus = "error"
	QueryTimeout      QueryStatus = "timeout"
	QueryRowLimit     QueryStatus = "row_limit"
	QueryNotWorkspace QueryStatus = "not_workspace"
)

type QueryResult struct {
	Columns  []QueryColumn `json:"columns"`
	Rows     [][]*string   `json:"rows"`
	Status   QueryStatus   `json:"status"`
	Message  string        `json:"message,omitempty"`
	Warnings []string      `json:"warnings,omitempty"`
}

type SQLReadRequest struct {
	Ref       string `json:"ref"`
	Query     string `json:"q"`
	Limit     int    `json:"limit,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type SQLWriteRequest struct {
	FromBranch string `json:"from_branch"`
	ToBranch   string `json:"to_branch"`
	Query      string `json:"q"`
}
