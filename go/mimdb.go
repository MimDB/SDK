// Package mimdb provides the Go SDK for the MimDB platform.
//
// Create a client with [NewClient] and use its accessor methods to reach
// domain-specific sub-clients (Platform, Auth, REST, Storage, etc.).
//
//	client := mimdb.NewClient("https://api.mimdb.dev", mimdb.Options{
//	    ProjectRef: "40891b0d",
//	    APIKey:     "your-api-key",
//	})
package mimdb

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/MimDB/SDK/go/internal/transport"
)

// Options configures a new [Client]. At minimum, either APIKey (for
// project-scoped access) or AdminSecret (for platform-level access) should be
// provided.
type Options struct {
	// ProjectRef is the short project identifier (e.g. "40891b0d"). Required
	// for project-scoped operations (auth, REST, storage, realtime).
	ProjectRef string

	// APIKey is the project-level API key sent as the "apikey" header. Used
	// for anon or service-role access within a specific project.
	APIKey string

	// AdminSecret is the platform admin secret used for organization and
	// project management endpoints. Sent as "Authorization: Bearer <secret>".
	AdminSecret string

	// HTTPClient allows callers to supply a custom *http.Client for timeouts,
	// proxies, or TLS configuration. If nil, a default client is used.
	HTTPClient *http.Client
}

// Client is the top-level entry point for the MimDB Go SDK. It holds shared
// configuration and lazily initializes domain-specific sub-clients on first
// access.
type Client struct {
	baseURL    string
	projectRef string
	options    Options
	transport  *transport.HTTPClient

	mu          sync.RWMutex
	accessToken string

	platformOnce sync.Once
	platform     *PlatformClient

	authOnce sync.Once
	auth     *AuthClient

	storageOnce sync.Once
	storage     *StorageClient

	realtimeOnce sync.Once
	realtime     *RealtimeClient

	vectorsOnce sync.Once
	vectors     *VectorClient

	cronOnce sync.Once
	cron     *CronClient

	sqlOnce sync.Once
	sql     *SQLClient

	introspectOnce sync.Once
	introspect     *IntrospectClient

	statsOnce sync.Once
	stats     *StatsClient
}

// NewClient creates a new MimDB client bound to the given base URL. The base
// URL should include the scheme and host (e.g. "https://api.mimdb.dev") and
// must not include a trailing slash.
//
// For project-scoped operations, set Options.ProjectRef and Options.APIKey.
// For platform-level management, set Options.AdminSecret.
func NewClient(baseURL string, opts Options) *Client {
	baseURL = strings.TrimRight(baseURL, "/")

	t := transport.NewHTTPClient(baseURL, transport.Config{
		APIKey:      opts.APIKey,
		AdminSecret: opts.AdminSecret,
		HTTPClient:  opts.HTTPClient,
	})

	return &Client{
		baseURL:    baseURL,
		projectRef: opts.ProjectRef,
		options:    opts,
		transport:  t,
	}
}

// SetAccessToken sets the user-level access token used for requests on behalf
// of an authenticated end-user. This overrides the AdminSecret for requests
// that include the token. It is safe to call from multiple goroutines.
func (c *Client) SetAccessToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = token
}

// getAccessToken returns the current user-level access token. It is safe to
// call from multiple goroutines.
func (c *Client) getAccessToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken
}

// From creates a new [QueryBuilder] targeting the given table for PostgREST
// SELECT queries. Chain filter, ordering, and pagination methods to build the
// query, then call Execute to run it.
//
//	var todos []Todo
//	err := client.From("todos").
//	    Select("id", "task").
//	    Eq("done", "false").
//	    Limit(10).
//	    Execute(ctx, &todos)
func (c *Client) From(table string) *QueryBuilder {
	return &QueryBuilder{
		client: c,
		table:  table,
	}
}

// RPC creates a [QueryBuilder] that calls a PostgREST remote procedure. The
// function name maps to the URL path /v1/rest/{ref}/rpc/{fn}. If params are
// provided, the first element is used as the JSON request body.
//
//	var result Summary
//	err := client.RPC("get_summary", map[string]any{"user_id": "abc"}).
//	    Execute(ctx, &result)
func (c *Client) RPC(fn string, params ...any) *QueryBuilder {
	qb := &QueryBuilder{
		client: c,
		rpcFn:  fn,
		method: "POST",
	}
	if len(params) > 0 {
		qb.body = params[0]
	}
	return qb
}

// Auth returns the [AuthClient] for user sign-up, sign-in, token refresh, and
// session management. The instance is created once and reused for all
// subsequent calls.
func (c *Client) Auth() *AuthClient {
	c.authOnce.Do(func() {
		c.auth = &AuthClient{client: c}
	})
	return c.auth
}

// Platform returns the [PlatformClient] for managing organizations, projects,
// API keys, and infrastructure resources. The instance is created once and
// reused for all subsequent calls.
func (c *Client) Platform() *PlatformClient {
	c.platformOnce.Do(func() {
		c.platform = &PlatformClient{client: c}
	})
	return c.platform
}

// Storage returns the [StorageClient] for managing buckets and objects (upload,
// download, delete, list, signed URLs, and public URLs). The instance is
// created once and reused for all subsequent calls.
func (c *Client) Storage() *StorageClient {
	c.storageOnce.Do(func() {
		c.storage = &StorageClient{client: c}
	})
	return c.storage
}

// Realtime returns the [RealtimeClient] for subscribing to database changes
// over a WebSocket connection. The instance is created once and reused for all
// subsequent calls.
func (c *Client) Realtime() *RealtimeClient {
	c.realtimeOnce.Do(func() {
		c.realtime = &RealtimeClient{
			client:    c,
			opts:      defaultRealtimeOptions(),
			state:     StateDisconnected,
			listeners: make(map[string][]listener),
			subs:      make(map[string]*Subscription),
		}
	})
	return c.realtime
}

// Vectors returns the [VectorClient] for managing vector-enabled tables,
// performing similarity searches, and building HNSW indexes. The instance is
// created once and reused for all subsequent calls.
func (c *Client) Vectors() *VectorClient {
	c.vectorsOnce.Do(func() {
		c.vectors = &VectorClient{client: c}
	})
	return c.vectors
}

// Cron returns the [CronClient] for managing scheduled cron jobs in the project
// database. The instance is created once and reused for all subsequent calls.
func (c *Client) Cron() *CronClient {
	c.cronOnce.Do(func() {
		c.cron = &CronClient{client: c}
	})
	return c.cron
}

// SQL returns the [SQLClient] for executing raw SQL queries against the project
// database. The instance is created once and reused for all subsequent calls.
func (c *Client) SQL() *SQLClient {
	c.sqlOnce.Do(func() {
		c.sql = &SQLClient{client: c}
	})
	return c.sql
}

// Introspect returns the [IntrospectClient] for schema introspection operations
// such as listing tables and inspecting column/index/foreign-key metadata. The
// instance is created once and reused for all subsequent calls.
func (c *Client) Introspect() *IntrospectClient {
	c.introspectOnce.Do(func() {
		c.introspect = &IntrospectClient{client: c}
	})
	return c.introspect
}

// Stats returns the [StatsClient] for retrieving query performance statistics
// from pg_stat_statements. The instance is created once and reused for all
// subsequent calls.
func (c *Client) Stats() *StatsClient {
	c.statsOnce.Do(func() {
		c.stats = &StatsClient{client: c}
	})
	return c.stats
}

// requireProjectRef returns an error if the client was not configured with a
// project ref. Project-scoped sub-clients call this before executing requests
// that require a project context.
func (c *Client) requireProjectRef() error {
	if c.projectRef == "" {
		return fmt.Errorf("ProjectRef is required for this operation")
	}
	return nil
}

// wrapTransportError converts an internal [transport.TransportError] to a
// public [*APIError] that SDK consumers can type-assert with [errors.As]. If
// the error is not a TransportError, it is returned unchanged.
func wrapTransportError(err error) error {
	if err == nil {
		return nil
	}

	var te *transport.TransportError
	if errors.As(err, &te) {
		return &APIError{
			Code:       te.Code,
			Message:    te.Message,
			Detail:     te.Detail,
			HTTPStatus: te.HTTPStatus,
			RequestID:  te.RequestID,
		}
	}
	return err
}
