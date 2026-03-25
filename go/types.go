package mimdb

import "time"

// ---------- Auth ----------

// User represents an authenticated user in the MimDB auth system.
type User struct {
	ID             string         `json:"id"`
	Email          *string        `json:"email,omitempty"`
	Phone          *string        `json:"phone,omitempty"`
	DisplayName    *string        `json:"display_name,omitempty"`
	AvatarURL      *string        `json:"avatar_url,omitempty"`
	EmailConfirmed bool           `json:"email_confirmed"`
	PhoneConfirmed bool           `json:"phone_confirmed"`
	TokenVersion   int            `json:"token_version"`
	AppMetadata    map[string]any `json:"app_metadata"`
	UserMetadata   map[string]any `json:"user_metadata"`
	BannedUntil    *time.Time     `json:"banned_until,omitempty"`
	LastSignInAt   *time.Time     `json:"last_sign_in_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// Tokens holds the JWT access token, refresh token, and expiry returned after
// a successful authentication flow.
type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// Session represents an active user session.
type Session struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	IPAddress  *string   `json:"ip_address,omitempty"`
	UserAgent  *string   `json:"user_agent,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// ---------- Realtime ----------

// RealtimeEvent represents a database change event received over a realtime
// subscription.
type RealtimeEvent struct {
	Type  EventType      `json:"type"`
	Table string         `json:"table"`
	New   map[string]any `json:"new"`
	Old   map[string]any `json:"old"`
}

// EventType identifies the kind of database mutation that triggered a realtime
// event.
type EventType string

const (
	// EventInsert indicates a new row was inserted.
	EventInsert EventType = "INSERT"

	// EventUpdate indicates an existing row was updated.
	EventUpdate EventType = "UPDATE"

	// EventDelete indicates a row was deleted.
	EventDelete EventType = "DELETE"

	// EventAll matches all event types in a subscription filter.
	EventAll EventType = "*"
)

// RealtimeError holds a structured error received over a realtime connection.
type RealtimeError struct {
	Code    string `json:"error_code"`
	Message string `json:"message"`
}

// ConnectionState represents the current state of a realtime WebSocket
// connection.
type ConnectionState string

const (
	// StateDisconnected means the connection is not established.
	StateDisconnected ConnectionState = "disconnected"

	// StateConnecting means the connection is being established.
	StateConnecting ConnectionState = "connecting"

	// StateConnected means the connection is active and healthy.
	StateConnected ConnectionState = "connected"

	// StateReconnecting means the connection was lost and is being restored.
	StateReconnecting ConnectionState = "reconnecting"
)

// ---------- REST ----------

// SortDirection controls the ordering of query results.
type SortDirection string

const (
	// Asc sorts results in ascending order.
	Asc SortDirection = "asc"

	// Desc sorts results in descending order.
	Desc SortDirection = "desc"
)

// ---------- Platform ----------

// Organization represents a MimDB organization that owns one or more projects.
type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Project represents a MimDB project within an organization.
type Project struct {
	ID         string         `json:"id"`
	OrgID      string         `json:"org_id"`
	Ref        string         `json:"ref"`
	Name       string         `json:"name"`
	SchemaName string         `json:"schema_name"`
	Region     string         `json:"region"`
	Status     string         `json:"status"`
	Settings   map[string]any `json:"settings"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// ProjectWithKeys bundles a project with its auto-generated API keys and
// database connection string, as returned by the create-project endpoint.
type ProjectWithKeys struct {
	Project            Project           `json:"project"`
	AnonKey            *APIKeyWithSecret `json:"anon_key"`
	ServiceRoleKey     *APIKeyWithSecret `json:"service_role_key"`
	DBConnectionString string            `json:"db_connection_string,omitempty"`
}

// APIKeyInfo holds metadata about an API key without exposing the secret.
type APIKeyInfo struct {
	ID        string     `json:"id"`
	ProjectID string     `json:"project_id"`
	Name      string     `json:"name"`
	KeyPrefix string     `json:"key_prefix"`
	Role      string     `json:"role"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// APIKeyWithSecret embeds APIKeyInfo and adds the raw (unhashed) key value.
// This is only returned once at key creation time.
type APIKeyWithSecret struct {
	APIKeyInfo
	RawKey string `json:"raw_key"`
}

// APIKeys holds the anon and service-role key strings for a project.
type APIKeys struct {
	AnonKey        string `json:"anon_key"`
	ServiceRoleKey string `json:"service_role_key"`
}

// ConnectionInfo describes the database connection endpoints for a project.
type ConnectionInfo struct {
	ProjectRef string        `json:"project_ref"`
	SchemaName string        `json:"schema_name"`
	DBPooled   *DBConnection `json:"db_pooled,omitempty"`
	DBDirect   *DBConnection `json:"db_direct,omitempty"`
}

// DBConnection holds the host, port, database, and user for a single database
// connection endpoint.
type DBConnection struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
}

// Extension represents a PostgreSQL extension available in or installed on a
// project database.
type Extension struct {
	Name            string `json:"name"`
	DisplayName     string `json:"display_name"`
	Description     string `json:"description"`
	PGName          string `json:"pg_name"`
	Installed       bool   `json:"installed"`
	Available       bool   `json:"available"`
	Preloaded       bool   `json:"preloaded"`
	RequiresPreload bool   `json:"requires_preload"`
	APIEnabled      bool   `json:"api_enabled"`
	Version         string `json:"version,omitempty"`
}

// AuthProvider represents an OAuth or social auth provider configured for a
// project.
type AuthProvider struct {
	ID                  string    `json:"id"`
	ProjectID           string    `json:"project_id"`
	Provider            string    `json:"provider"`
	ClientID            string    `json:"client_id"`
	Scopes              []string  `json:"scopes"`
	AllowedRedirectURLs []string  `json:"allowed_redirect_urls"`
	Enabled             bool      `json:"enabled"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ---------- Storage ----------

// Bucket represents a storage bucket within a project.
type Bucket struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	Name          string    `json:"name"`
	Public        bool      `json:"public"`
	FileSizeLimit *int64    `json:"file_size_limit,omitempty"`
	AllowedTypes  []string  `json:"allowed_types,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// StorageObject represents a file stored in a bucket.
type StorageObject struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	MimeType  string    `json:"mime_type"`
	CreatedAt time.Time `json:"created_at"`
}

// ---------- Vectors ----------

// VectorTable describes a table configured for vector similarity search.
type VectorTable struct {
	Name       string `json:"name"`
	Dimensions int    `json:"dimensions"`
}

// VectorSearchResult holds a single result from a vector similarity search,
// including the similarity score and the associated row data.
type VectorSearchResult struct {
	ID         string         `json:"id"`
	Similarity float64        `json:"similarity"`
	Data       map[string]any `json:"data"`
}

// ---------- Cron ----------

// CronJob represents a scheduled cron job in the project database.
type CronJob struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Schedule  string     `json:"schedule"`
	Command   string     `json:"command"`
	Active    bool       `json:"active"`
	Database  string     `json:"database,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// CronJobRun represents a single execution of a cron job.
type CronJobRun struct {
	RunID         int64      `json:"run_id"`
	JobID         int64      `json:"job_id"`
	Command       string     `json:"command"`
	Status        string     `json:"status"`
	ReturnMessage string     `json:"return_message"`
	StartTime     *time.Time `json:"start_time"`
	EndTime       *time.Time `json:"end_time"`
}

// ---------- SQL ----------

// SQLResult holds the result of an SQL query execution, including column
// metadata, rows, and timing information.
type SQLResult struct {
	Columns    []ResultColumn   `json:"columns"`
	Rows       []map[string]any `json:"rows"`
	RowCount   int              `json:"row_count"`
	Truncated  bool             `json:"truncated"`
	ExecTimeMs float64          `json:"execution_time_ms"`
	CommandTag string           `json:"command_tag"`
}

// ResultColumn describes a single column in an SQL result set.
type ResultColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ---------- Introspection ----------

// TableSummary provides a high-level overview of a database table including
// row estimates and disk size.
type TableSummary struct {
	Name        string  `json:"name"`
	Schema      string  `json:"schema"`
	RowEstimate int64   `json:"row_estimate"`
	SizeBytes   int64   `json:"size_bytes"`
	Comment     *string `json:"comment"`
}

// TableDetail provides full introspection data for a single table, including
// columns, keys, and indexes.
type TableDetail struct {
	Name        string       `json:"name"`
	Schema      string       `json:"schema"`
	Columns     []Column     `json:"columns"`
	PrimaryKey  []string     `json:"primary_key"`
	ForeignKeys []ForeignKey `json:"foreign_keys"`
	Indexes     []Index      `json:"indexes"`
	RowEstimate int64        `json:"row_estimate"`
	SizeBytes   int64        `json:"size_bytes"`
	Comment     *string      `json:"comment"`
}

// Column describes a single column within a database table.
type Column struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Nullable     bool    `json:"nullable"`
	DefaultValue *string `json:"default_value"`
	IsPrimaryKey bool    `json:"is_primary_key"`
	Comment      *string `json:"comment"`
	OrdinalPos   int     `json:"ordinal_position"`
}

// ForeignKey describes a foreign key constraint between tables.
type ForeignKey struct {
	Name           string   `json:"name"`
	Columns        []string `json:"columns"`
	ForeignSchema  string   `json:"foreign_schema"`
	ForeignTable   string   `json:"foreign_table"`
	ForeignColumns []string `json:"foreign_columns"`
}

// Index describes a database index on a table.
type Index struct {
	Name      string   `json:"name"`
	Columns   []string `json:"columns"`
	IsUnique  bool     `json:"is_unique"`
	IsPrimary bool     `json:"is_primary"`
	Type      string   `json:"type"`
}

// ---------- Query Stats ----------

// QueryStat holds statistics for a single query from pg_stat_statements.
type QueryStat struct {
	QueryID         int64   `json:"queryid"`
	Query           string  `json:"query"`
	Calls           int64   `json:"calls"`
	MeanExecTimeMs  float64 `json:"mean_exec_time_ms"`
	TotalExecTimeMs float64 `json:"total_exec_time_ms"`
}

// QueryStatsResponse holds the full query statistics response including the
// list of queries and an optional stats reset timestamp. The backend sends
// stats_reset as a text string rather than an RFC3339 time.
type QueryStatsResponse struct {
	Queries      []QueryStat `json:"queries"`
	TotalQueries int         `json:"total_queries"`
	StatsReset   *string     `json:"stats_reset,omitempty"`
}

// ---------- Common Options ----------

// ListOptions holds pagination and filtering parameters for list endpoints.
type ListOptions struct {
	// Cursor is an opaque token for cursor-based pagination.
	Cursor string

	// Limit controls the maximum number of items returned per page.
	Limit int

	// Prefix filters results by name prefix (used by storage object listing).
	Prefix string
}

// UploadOptions configures file upload behavior.
type UploadOptions struct {
	// ContentType overrides the MIME type for the uploaded file.
	ContentType string
}
