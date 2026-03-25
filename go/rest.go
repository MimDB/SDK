package mimdb

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/MimDB/SDK/go/internal/transport"
)

// restBasePath is the URL prefix template for PostgREST proxy endpoints.
// The {ref} and {table} segments are filled in at build time.
const restBasePath = "/v1/rest"

// filter holds a single PostgREST column filter such as "eq.value" applied to
// a specific column name.
type filter struct {
	column string
	expr   string
}

// orderClause holds a single ordering directive such as "created_at.desc".
type orderClause struct {
	column    string
	direction SortDirection
}

// QueryBuilder constructs PostgREST-compatible queries using a fluent chainable
// API. It supports SELECT, INSERT, UPDATE, DELETE, and RPC operations. The
// builder accumulates column selections, filters, ordering, pagination, and
// mutation body, then executes the query via the transport layer.
//
// Obtain a QueryBuilder from [Client.From] for table operations or [Client.RPC]
// for remote procedure calls:
//
//	var todos []Todo
//	err := client.From("todos").
//	    Select("id", "task", "done").
//	    Eq("done", "false").
//	    Order("created_at", Desc).
//	    Limit(10).
//	    Execute(ctx, &todos)
type QueryBuilder struct {
	client  *Client
	table   string
	columns []string
	filters []filter
	orders  []orderClause
	limit   *int
	offset  *int
	single  bool
	method  string // HTTP method override (POST, PATCH, DELETE); empty = GET
	body    any    // request body for mutations (Insert, Update, RPC)
	rpcFn   string // RPC function name; when set, path targets /rpc/{fn}
}

// Select specifies which columns to retrieve. If called with no arguments,
// all columns are returned (PostgREST default). Calling Select multiple times
// replaces the previous selection.
//
//	qb.Select("id", "name", "email")
func (q *QueryBuilder) Select(columns ...string) *QueryBuilder {
	q.columns = columns
	return q
}

// Eq adds an equality filter: column = value.
//
//	qb.Eq("status", "active")  // status=eq.active
func (q *QueryBuilder) Eq(column, value string) *QueryBuilder {
	q.filters = append(q.filters, filter{column: column, expr: "eq." + value})
	return q
}

// Neq adds a not-equal filter: column != value.
//
//	qb.Neq("status", "archived")  // status=neq.archived
func (q *QueryBuilder) Neq(column, value string) *QueryBuilder {
	q.filters = append(q.filters, filter{column: column, expr: "neq." + value})
	return q
}

// Gt adds a greater-than filter: column > value.
//
//	qb.Gt("age", "18")  // age=gt.18
func (q *QueryBuilder) Gt(column, value string) *QueryBuilder {
	q.filters = append(q.filters, filter{column: column, expr: "gt." + value})
	return q
}

// Lt adds a less-than filter: column < value.
//
//	qb.Lt("age", "65")  // age=lt.65
func (q *QueryBuilder) Lt(column, value string) *QueryBuilder {
	q.filters = append(q.filters, filter{column: column, expr: "lt." + value})
	return q
}

// Gte adds a greater-than-or-equal filter: column >= value.
//
//	qb.Gte("score", "90")  // score=gte.90
func (q *QueryBuilder) Gte(column, value string) *QueryBuilder {
	q.filters = append(q.filters, filter{column: column, expr: "gte." + value})
	return q
}

// Lte adds a less-than-or-equal filter: column <= value.
//
//	qb.Lte("score", "10")  // score=lte.10
func (q *QueryBuilder) Lte(column, value string) *QueryBuilder {
	q.filters = append(q.filters, filter{column: column, expr: "lte." + value})
	return q
}

// Like adds a LIKE pattern filter (case-sensitive).
//
//	qb.Like("name", "%foo%")  // name=like.%foo%
func (q *QueryBuilder) Like(column, pattern string) *QueryBuilder {
	q.filters = append(q.filters, filter{column: column, expr: "like." + pattern})
	return q
}

// ILike adds an ILIKE pattern filter (case-insensitive).
//
//	qb.ILike("name", "%foo%")  // name=ilike.%foo%
func (q *QueryBuilder) ILike(column, pattern string) *QueryBuilder {
	q.filters = append(q.filters, filter{column: column, expr: "ilike." + pattern})
	return q
}

// In adds a membership filter: column IN (values...).
//
//	qb.In("id", []string{"1", "2", "3"})  // id=in.(1,2,3)
func (q *QueryBuilder) In(column string, values []string) *QueryBuilder {
	expr := "in.(" + strings.Join(values, ",") + ")"
	q.filters = append(q.filters, filter{column: column, expr: expr})
	return q
}

// Is adds an IS filter, typically used for null checks or boolean tests.
//
//	qb.Is("deleted_at", "null")  // deleted_at=is.null
func (q *QueryBuilder) Is(column, value string) *QueryBuilder {
	q.filters = append(q.filters, filter{column: column, expr: "is." + value})
	return q
}

// Order appends an ordering directive. Multiple calls accumulate into a
// comma-separated PostgREST order parameter.
//
//	qb.Order("created_at", Desc).Order("name", Asc)
//	// order=created_at.desc,name.asc
func (q *QueryBuilder) Order(column string, dir SortDirection) *QueryBuilder {
	q.orders = append(q.orders, orderClause{column: column, direction: dir})
	return q
}

// Limit sets the maximum number of rows to return.
//
//	qb.Limit(10)  // limit=10
func (q *QueryBuilder) Limit(n int) *QueryBuilder {
	q.limit = &n
	return q
}

// Offset sets the number of rows to skip before returning results. Typically
// used in combination with [QueryBuilder.Limit] for pagination.
//
//	qb.Limit(10).Offset(20)  // limit=10&offset=20
func (q *QueryBuilder) Offset(n int) *QueryBuilder {
	q.offset = &n
	return q
}

// Single configures the query to return a single JSON object instead of an
// array. This adds the PostgREST headers:
//   - Accept: application/vnd.pgrst.object+json
//   - Prefer: return=representation
//
// If the query returns zero or more than one row, PostgREST will respond with
// an error.
func (q *QueryBuilder) Single() *QueryBuilder {
	q.single = true
	return q
}

// Insert configures the query to perform a POST insert operation. The data
// parameter is serialized as JSON in the request body. Use [QueryBuilder.Single]
// to return the created row as a single object instead of an array.
//
//	var created []Todo
//	err := client.From("todos").
//	    Insert(map[string]any{"task": "buy milk"}).
//	    Execute(ctx, &created)
func (q *QueryBuilder) Insert(data any) *QueryBuilder {
	q.method = http.MethodPost
	q.body = data
	return q
}

// Update configures the query to perform a PATCH update operation. The data
// parameter is serialized as JSON in the request body. Chain filter methods
// (Eq, Gt, etc.) to narrow which rows are updated.
//
//	var updated []Todo
//	err := client.From("todos").
//	    Eq("id", "42").
//	    Update(map[string]any{"done": true}).
//	    Execute(ctx, &updated)
func (q *QueryBuilder) Update(data any) *QueryBuilder {
	q.method = http.MethodPatch
	q.body = data
	return q
}

// Delete configures the query to perform a DELETE operation. Chain filter
// methods (Eq, Gt, etc.) to narrow which rows are deleted.
//
//	err := client.From("todos").
//	    Eq("id", "42").
//	    Delete().
//	    Execute(ctx, nil)
func (q *QueryBuilder) Delete() *QueryBuilder {
	q.method = http.MethodDelete
	return q
}

// Execute runs the built query against the PostgREST proxy endpoint and
// decodes the response into dest. For normal queries, dest should be a pointer
// to a slice. For Single() queries, dest should be a pointer to a struct.
// For mutations (Insert, Update, Delete), the appropriate HTTP method and
// request body are used automatically.
//
// The method uses [transport.HTTPClient.DoJSON] (raw JSON mode) because the
// REST proxy returns plain JSON without the MimDB envelope.
func (q *QueryBuilder) Execute(ctx context.Context, dest any) error {
	if err := q.client.requireProjectRef(); err != nil {
		return err
	}

	path, params := q.buildURL()
	fullPath := path + "?" + params.Encode()

	method := q.resolveMethod()
	opts := q.buildRequestOptions()

	err := q.client.transport.DoJSON(ctx, method, fullPath, q.body, dest, opts)
	return wrapTransportError(err)
}

// resolveMethod returns the HTTP method for the query. If a mutation method
// (POST, PATCH, DELETE) was explicitly set via Insert, Update, or Delete, that
// method is used. Otherwise, GET is the default for SELECT queries.
func (q *QueryBuilder) resolveMethod() string {
	if q.method != "" {
		return q.method
	}
	return http.MethodGet
}

// buildURL constructs the URL path and query parameters from the accumulated
// query builder state. For table operations the path follows the format
// /v1/rest/{ref}/{table}. For RPC calls it uses /v1/rest/{ref}/rpc/{fn}.
// The parameters include select, filters, order, limit, and offset.
func (q *QueryBuilder) buildURL() (string, url.Values) {
	var path string
	if q.rpcFn != "" {
		path = fmt.Sprintf("%s/%s/rpc/%s", restBasePath, q.client.projectRef, q.rpcFn)
	} else {
		path = fmt.Sprintf("%s/%s/%s", restBasePath, q.client.projectRef, q.table)
	}

	params := url.Values{}

	if len(q.columns) > 0 {
		params.Set("select", strings.Join(q.columns, ","))
	}

	for _, f := range q.filters {
		params.Add(f.column, f.expr)
	}

	if len(q.orders) > 0 {
		parts := make([]string, len(q.orders))
		for i, o := range q.orders {
			parts[i] = o.column + "." + string(o.direction)
		}
		params.Set("order", strings.Join(parts, ","))
	}

	if q.limit != nil {
		params.Set("limit", strconv.Itoa(*q.limit))
	}

	if q.offset != nil {
		params.Set("offset", strconv.Itoa(*q.offset))
	}

	return path, params
}

// buildRequestOptions creates the transport.RequestOptions for the query,
// including Single() headers and the user access token when set.
func (q *QueryBuilder) buildRequestOptions() transport.RequestOptions {
	opts := transport.RequestOptions{}

	if token := q.client.getAccessToken(); token != "" {
		opts.AccessToken = token
	}

	if q.single {
		if opts.Headers == nil {
			opts.Headers = make(map[string]string)
		}
		opts.Headers["Accept"] = "application/vnd.pgrst.object+json"
		opts.Headers["Prefer"] = "return=representation"
	}

	return opts
}

// ---------------------------------------------------------------------------
// Generics sugar
// ---------------------------------------------------------------------------

// Query describes a typed query for the generics helper [Select]. It specifies
// which columns to return and optional PostgREST filters.
type Query struct {
	// Columns lists the column names to select. An empty slice selects all.
	Columns []string

	// Filters lists PostgREST column filters to apply.
	Filters []Filter
}

// Filter represents a single PostgREST column filter such as "eq.value" applied
// to a specific column name. Use the constructor functions [Eq], [Neq], etc. to
// create filters.
type Filter struct {
	// Column is the database column name to filter on.
	Column string

	// Operator is the PostgREST operator (e.g. "eq", "neq", "gt").
	Operator string

	// Value is the comparison value.
	Value string
}

// Eq creates an equality filter: column = value.
//
//	Eq("status", "active")
func Eq(column, value string) Filter {
	return Filter{Column: column, Operator: "eq", Value: value}
}

// Neq creates a not-equal filter: column != value.
//
//	Neq("status", "archived")
func Neq(column, value string) Filter {
	return Filter{Column: column, Operator: "neq", Value: value}
}

// Gt creates a greater-than filter: column > value.
//
//	Gt("age", "18")
func Gt(column, value string) Filter {
	return Filter{Column: column, Operator: "gt", Value: value}
}

// Lt creates a less-than filter: column < value.
//
//	Lt("age", "65")
func Lt(column, value string) Filter {
	return Filter{Column: column, Operator: "lt", Value: value}
}

// Gte creates a greater-than-or-equal filter: column >= value.
//
//	Gte("score", "90")
func Gte(column, value string) Filter {
	return Filter{Column: column, Operator: "gte", Value: value}
}

// Lte creates a less-than-or-equal filter: column <= value.
//
//	Lte("score", "10")
func Lte(column, value string) Filter {
	return Filter{Column: column, Operator: "lte", Value: value}
}

// Select performs a typed SELECT query and returns a slice of T. It builds a
// [QueryBuilder] from the [Query] struct and calls Execute, returning the
// deserialized results directly.
//
//	type Todo struct {
//	    ID   int    `json:"id"`
//	    Task string `json:"task"`
//	}
//
//	todos, err := mimdb.Select[Todo](client, "todos", mimdb.Query{
//	    Columns: []string{"id", "task"},
//	    Filters: []mimdb.Filter{mimdb.Eq("done", "false")},
//	})
func Select[T any](client *Client, table string, q Query) ([]T, error) {
	qb := client.From(table).Select(q.Columns...)

	for _, f := range q.Filters {
		qb.filters = append(qb.filters, filter{
			column: f.Column,
			expr:   f.Operator + "." + f.Value,
		})
	}

	var result []T
	if err := qb.Execute(context.Background(), &result); err != nil {
		return nil, err
	}
	return result, nil
}
