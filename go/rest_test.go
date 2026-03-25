package mimdb

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestQueryBuilder_SelectURL verifies that the query builder produces the
// correct PostgREST-compatible URL path and query parameters for a typical
// SELECT query with filters, ordering, and pagination.
func TestQueryBuilder_SelectURL(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "40891b0d",
		APIKey:     "test-key",
	})

	qb := client.From("todos").
		Select("id", "task").
		Eq("done", "false").
		Order("created_at", Desc).
		Limit(10)

	path, params := qb.buildURL()

	if path != "/v1/rest/40891b0d/todos" {
		t.Errorf("path = %q, want /v1/rest/40891b0d/todos", path)
	}
	if got := params.Get("select"); got != "id,task" {
		t.Errorf("select = %q, want %q", got, "id,task")
	}
	if got := params.Get("done"); got != "eq.false" {
		t.Errorf("done filter = %q, want %q", got, "eq.false")
	}
	if got := params.Get("order"); got != "created_at.desc" {
		t.Errorf("order = %q, want %q", got, "created_at.desc")
	}
	if got := params.Get("limit"); got != "10" {
		t.Errorf("limit = %q, want %q", got, "10")
	}
}

// TestQueryBuilder_AllFilters verifies that each filter method generates the
// correct PostgREST operator syntax in query parameters.
func TestQueryBuilder_AllFilters(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "ref1",
		APIKey:     "k",
	})

	tests := []struct {
		name   string
		build  func() *QueryBuilder
		column string
		want   string
	}{
		{
			name:   "Eq",
			build:  func() *QueryBuilder { return client.From("t").Eq("a", "b") },
			column: "a",
			want:   "eq.b",
		},
		{
			name:   "Neq",
			build:  func() *QueryBuilder { return client.From("t").Neq("a", "b") },
			column: "a",
			want:   "neq.b",
		},
		{
			name:   "Gt",
			build:  func() *QueryBuilder { return client.From("t").Gt("age", "18") },
			column: "age",
			want:   "gt.18",
		},
		{
			name:   "Lt",
			build:  func() *QueryBuilder { return client.From("t").Lt("age", "18") },
			column: "age",
			want:   "lt.18",
		},
		{
			name:   "Gte",
			build:  func() *QueryBuilder { return client.From("t").Gte("score", "90") },
			column: "score",
			want:   "gte.90",
		},
		{
			name:   "Lte",
			build:  func() *QueryBuilder { return client.From("t").Lte("score", "10") },
			column: "score",
			want:   "lte.10",
		},
		{
			name:   "Like",
			build:  func() *QueryBuilder { return client.From("t").Like("name", "%foo%") },
			column: "name",
			want:   "like.%foo%",
		},
		{
			name:   "ILike",
			build:  func() *QueryBuilder { return client.From("t").ILike("name", "%foo%") },
			column: "name",
			want:   "ilike.%foo%",
		},
		{
			name:   "In",
			build:  func() *QueryBuilder { return client.From("t").In("id", []string{"1", "2", "3"}) },
			column: "id",
			want:   "in.(1,2,3)",
		},
		{
			name:   "Is",
			build:  func() *QueryBuilder { return client.From("t").Is("deleted_at", "null") },
			column: "deleted_at",
			want:   "is.null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qb := tt.build()
			_, params := qb.buildURL()
			got := params.Get(tt.column)
			if got != tt.want {
				t.Errorf("%s filter: %s = %q, want %q", tt.name, tt.column, got, tt.want)
			}
		})
	}
}

// TestQueryBuilder_Execute verifies that the query builder sends an HTTP GET to
// the correct URL and deserializes a plain JSON array response into the
// destination slice.
func TestQueryBuilder_Execute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}

		wantPath := "/v1/rest/ref1/todos"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}

		q := r.URL.Query()
		if got := q.Get("select"); got != "id,task" {
			t.Errorf("select = %q, want %q", got, "id,task")
		}
		if got := q.Get("done"); got != "eq.false" {
			t.Errorf("done filter = %q, want %q", got, "eq.false")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":1,"task":"test"},{"id":2,"task":"other"}]`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "ref1",
		APIKey:     "test-key",
	})

	type Todo struct {
		ID   int    `json:"id"`
		Task string `json:"task"`
	}

	var todos []Todo
	err := client.From("todos").
		Select("id", "task").
		Eq("done", "false").
		Execute(context.Background(), &todos)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(todos) != 2 {
		t.Fatalf("len(todos) = %d, want 2", len(todos))
	}
	if todos[0].ID != 1 || todos[0].Task != "test" {
		t.Errorf("todos[0] = %+v, want {ID:1 Task:test}", todos[0])
	}
	if todos[1].ID != 2 || todos[1].Task != "other" {
		t.Errorf("todos[1] = %+v, want {ID:2 Task:other}", todos[1])
	}
}

// TestQueryBuilder_SingleItem verifies that Single() adds the correct Accept
// and Prefer headers and correctly deserializes a single JSON object response.
func TestQueryBuilder_SingleItem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}

		accept := r.Header.Get("Accept")
		if accept != "application/vnd.pgrst.object+json" {
			t.Errorf("Accept = %q, want %q", accept, "application/vnd.pgrst.object+json")
		}

		prefer := r.Header.Get("Prefer")
		if prefer != "return=representation" {
			t.Errorf("Prefer = %q, want %q", prefer, "return=representation")
		}

		w.Header().Set("Content-Type", "application/vnd.pgrst.object+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"task":"single item"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "ref1",
		APIKey:     "test-key",
	})

	type Todo struct {
		ID   int    `json:"id"`
		Task string `json:"task"`
	}

	var todo Todo
	err := client.From("todos").
		Select("id", "task").
		Eq("id", "1").
		Single().
		Execute(context.Background(), &todo)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if todo.ID != 1 {
		t.Errorf("todo.ID = %d, want 1", todo.ID)
	}
	if todo.Task != "single item" {
		t.Errorf("todo.Task = %q, want %q", todo.Task, "single item")
	}
}

// TestQueryBuilder_RequiresProjectRef verifies that Execute returns an error
// when the client has no ProjectRef configured.
func TestQueryBuilder_RequiresProjectRef(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		AdminSecret: "admin-only",
	})

	var dest []map[string]any
	err := client.From("todos").
		Select("id").
		Execute(context.Background(), &dest)

	if err == nil {
		t.Fatal("expected error for missing ProjectRef")
	}

	if got := err.Error(); got != "ProjectRef is required for this operation" {
		t.Errorf("error = %q, want ProjectRef error message", got)
	}
}

// TestQueryBuilder_PostgRESTError verifies that a non-2xx response with
// PostgREST error format is parsed into an *APIError with the correct fields.
func TestQueryBuilder_PostgRESTError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"code":    "PGRST204",
			"message": "Column not found",
			"details": "Column 'foo' does not exist",
			"hint":    "Check the column name",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "ref1",
		APIKey:     "test-key",
	})

	var dest []map[string]any
	err := client.From("todos").
		Select("foo").
		Execute(context.Background(), &dest)

	if err == nil {
		t.Fatal("expected error for PostgREST 400")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "PGRST204" {
		t.Errorf("Code = %q, want %q", apiErr.Code, "PGRST204")
	}
	if apiErr.Message != "Column not found" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "Column not found")
	}
	if apiErr.Detail != "Column 'foo' does not exist" {
		t.Errorf("Detail = %q, want %q", apiErr.Detail, "Column 'foo' does not exist")
	}
	if apiErr.HTTPStatus != http.StatusBadRequest {
		t.Errorf("HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusBadRequest)
	}
}

// TestQueryBuilder_OrderMultiple verifies that multiple Order calls produce a
// comma-separated order parameter.
func TestQueryBuilder_OrderMultiple(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "ref1",
		APIKey:     "k",
	})

	qb := client.From("t").
		Order("created_at", Desc).
		Order("name", Asc)

	_, params := qb.buildURL()
	got := params.Get("order")
	if got != "created_at.desc,name.asc" {
		t.Errorf("order = %q, want %q", got, "created_at.desc,name.asc")
	}
}

// TestQueryBuilder_Offset verifies that the offset query parameter is set.
func TestQueryBuilder_Offset(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "ref1",
		APIKey:     "k",
	})

	qb := client.From("t").Limit(10).Offset(20)
	_, params := qb.buildURL()

	if got := params.Get("limit"); got != "10" {
		t.Errorf("limit = %q, want %q", got, "10")
	}
	if got := params.Get("offset"); got != "20" {
		t.Errorf("offset = %q, want %q", got, "20")
	}
}

// TestQueryBuilder_SelectStar verifies that calling Select with no arguments
// does not add a select parameter, letting PostgREST return all columns.
func TestQueryBuilder_SelectStar(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "ref1",
		APIKey:     "k",
	})

	qb := client.From("t").Select()
	_, params := qb.buildURL()

	if params.Has("select") {
		t.Errorf("select should not be set for empty Select(), got %q", params.Get("select"))
	}
}

// TestQueryBuilder_FullURL verifies the complete URL string built from the path
// and query parameters, ensuring proper encoding.
func TestQueryBuilder_FullURL(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "abc123",
		APIKey:     "k",
	})

	qb := client.From("tasks").
		Select("id", "title", "done").
		Eq("done", "false").
		Order("title", Asc).
		Limit(5).
		Offset(0)

	path, params := qb.buildURL()
	fullURL := path + "?" + params.Encode()

	parsed, err := url.Parse(fullURL)
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}

	if parsed.Path != "/v1/rest/abc123/tasks" {
		t.Errorf("path = %q", parsed.Path)
	}

	q := parsed.Query()
	if q.Get("select") != "id,title,done" {
		t.Errorf("select = %q", q.Get("select"))
	}
	if q.Get("done") != "eq.false" {
		t.Errorf("done = %q", q.Get("done"))
	}
	if q.Get("order") != "title.asc" {
		t.Errorf("order = %q", q.Get("order"))
	}
	if q.Get("limit") != "5" {
		t.Errorf("limit = %q", q.Get("limit"))
	}
	if q.Get("offset") != "0" {
		t.Errorf("offset = %q", q.Get("offset"))
	}
}

// TestQueryBuilder_MultipleFiltersOnSameColumn verifies that multiple filters
// on the same column are preserved (PostgREST supports this for range queries).
func TestQueryBuilder_MultipleFiltersOnSameColumn(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "ref1",
		APIKey:     "k",
	})

	qb := client.From("t").
		Gte("age", "18").
		Lte("age", "65")

	_, params := qb.buildURL()

	// Both filters on "age" should be present.
	values := params["age"]
	if len(values) != 2 {
		t.Fatalf("expected 2 values for age, got %d: %v", len(values), values)
	}

	found := map[string]bool{}
	for _, v := range values {
		found[v] = true
	}
	if !found["gte.18"] {
		t.Error("missing gte.18 filter for age")
	}
	if !found["lte.65"] {
		t.Error("missing lte.65 filter for age")
	}
}

// TestQueryBuilder_AccessToken verifies that the user access token is forwarded
// to the transport layer when set on the client.
func TestQueryBuilder_AccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer user-token-abc" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer user-token-abc")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "ref1",
		APIKey:     "test-key",
	})
	client.SetAccessToken("user-token-abc")

	var dest []map[string]any
	err := client.From("todos").
		Select("id").
		Execute(context.Background(), &dest)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestQueryBuilder_Insert verifies that Insert sets the HTTP method to POST,
// sends the body as JSON, and deserializes the created item from the response.
func TestQueryBuilder_Insert(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}

		wantPath := "/v1/rest/ref1/todos"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		defer r.Body.Close()

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal body: %v", err)
		}
		if payload["task"] != "buy milk" {
			t.Errorf("body task = %v, want %q", payload["task"], "buy milk")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`[{"id":42,"task":"buy milk","done":false}]`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "ref1",
		APIKey:     "test-key",
	})

	type Todo struct {
		ID   int    `json:"id"`
		Task string `json:"task"`
		Done bool   `json:"done"`
	}

	var created []Todo
	err := client.From("todos").
		Insert(map[string]any{"task": "buy milk", "done": false}).
		Execute(context.Background(), &created)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("len(created) = %d, want 1", len(created))
	}
	if created[0].ID != 42 {
		t.Errorf("created[0].ID = %d, want 42", created[0].ID)
	}
	if created[0].Task != "buy milk" {
		t.Errorf("created[0].Task = %q, want %q", created[0].Task, "buy milk")
	}
}

// TestQueryBuilder_InsertSingle verifies that Insert combined with Single sends
// the Prefer: return=representation and Accept: application/vnd.pgrst.object+json
// headers.
func TestQueryBuilder_InsertSingle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}

		accept := r.Header.Get("Accept")
		if accept != "application/vnd.pgrst.object+json" {
			t.Errorf("Accept = %q, want %q", accept, "application/vnd.pgrst.object+json")
		}

		prefer := r.Header.Get("Prefer")
		if prefer != "return=representation" {
			t.Errorf("Prefer = %q, want %q", prefer, "return=representation")
		}

		w.Header().Set("Content-Type", "application/vnd.pgrst.object+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42,"task":"buy milk","done":false}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "ref1",
		APIKey:     "test-key",
	})

	type Todo struct {
		ID   int    `json:"id"`
		Task string `json:"task"`
		Done bool   `json:"done"`
	}

	var created Todo
	err := client.From("todos").
		Insert(map[string]any{"task": "buy milk", "done": false}).
		Single().
		Execute(context.Background(), &created)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID != 42 {
		t.Errorf("created.ID = %d, want 42", created.ID)
	}
	if created.Task != "buy milk" {
		t.Errorf("created.Task = %q, want %q", created.Task, "buy milk")
	}
}

// TestQueryBuilder_Update verifies that Update sets the HTTP method to PATCH,
// sends the body as JSON, and includes filters in query parameters.
func TestQueryBuilder_Update(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %q, want PATCH", r.Method)
		}

		wantPath := "/v1/rest/ref1/todos"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}

		// Verify filter in query params.
		if got := r.URL.Query().Get("id"); got != "eq.42" {
			t.Errorf("id filter = %q, want %q", got, "eq.42")
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		defer r.Body.Close()

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal body: %v", err)
		}
		if payload["done"] != true {
			t.Errorf("body done = %v, want true", payload["done"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":42,"task":"buy milk","done":true}]`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "ref1",
		APIKey:     "test-key",
	})

	type Todo struct {
		ID   int    `json:"id"`
		Task string `json:"task"`
		Done bool   `json:"done"`
	}

	var updated []Todo
	err := client.From("todos").
		Eq("id", "42").
		Update(map[string]any{"done": true}).
		Execute(context.Background(), &updated)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updated) != 1 {
		t.Fatalf("len(updated) = %d, want 1", len(updated))
	}
	if !updated[0].Done {
		t.Errorf("updated[0].Done = false, want true")
	}
}

// TestQueryBuilder_Delete verifies that Delete sets the HTTP method to DELETE
// and includes filters in query parameters.
func TestQueryBuilder_Delete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}

		wantPath := "/v1/rest/ref1/todos"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}

		// Verify filter in query params.
		if got := r.URL.Query().Get("id"); got != "eq.42" {
			t.Errorf("id filter = %q, want %q", got, "eq.42")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "ref1",
		APIKey:     "test-key",
	})

	err := client.From("todos").
		Eq("id", "42").
		Delete().
		Execute(context.Background(), nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestQueryBuilder_RPC verifies that RPC posts to /rpc/{fn} with a JSON body.
func TestQueryBuilder_RPC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}

		wantPath := "/v1/rest/ref1/rpc/get_summary"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		defer r.Body.Close()

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal body: %v", err)
		}
		if payload["user_id"] != "abc" {
			t.Errorf("body user_id = %v, want %q", payload["user_id"], "abc")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total":5,"active":3}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "ref1",
		APIKey:     "test-key",
	})

	type Summary struct {
		Total  int `json:"total"`
		Active int `json:"active"`
	}

	var result Summary
	err := client.RPC("get_summary", map[string]any{"user_id": "abc"}).
		Execute(context.Background(), &result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("result.Total = %d, want 5", result.Total)
	}
	if result.Active != 3 {
		t.Errorf("result.Active = %d, want 3", result.Active)
	}
}

// TestQueryBuilder_RPCNoParams verifies that RPC without params sends POST
// to /rpc/{fn} with no body.
func TestQueryBuilder_RPCNoParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}

		wantPath := "/v1/rest/ref1/rpc/ping"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		defer r.Body.Close()

		// Body should be empty or not provided.
		if len(body) > 0 {
			t.Errorf("expected empty body, got %q", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`"pong"`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "ref1",
		APIKey:     "test-key",
	})

	var result string
	err := client.RPC("ping").
		Execute(context.Background(), &result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "pong" {
		t.Errorf("result = %q, want %q", result, "pong")
	}
}

// TestSelect_Generics verifies the typed generic Select[T] helper produces
// the correct query and deserializes results into the specified type.
func TestSelect_Generics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}

		wantPath := "/v1/rest/ref1/todos"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}

		q := r.URL.Query()
		if got := q.Get("select"); got != "id,task,done" {
			t.Errorf("select = %q, want %q", got, "id,task,done")
		}
		if got := q.Get("done"); got != "eq.false" {
			t.Errorf("done filter = %q, want %q", got, "eq.false")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":1,"task":"test","done":false},{"id":2,"task":"other","done":false}]`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "ref1",
		APIKey:     "test-key",
	})

	type Todo struct {
		ID   int    `json:"id"`
		Task string `json:"task"`
		Done bool   `json:"done"`
	}

	todos, err := Select[Todo](client, "todos", Query{
		Columns: []string{"id", "task", "done"},
		Filters: []Filter{
			Eq("done", "false"),
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(todos) != 2 {
		t.Fatalf("len(todos) = %d, want 2", len(todos))
	}
	if todos[0].ID != 1 || todos[0].Task != "test" {
		t.Errorf("todos[0] = %+v, want {ID:1 Task:test Done:false}", todos[0])
	}
	if todos[1].ID != 2 || todos[1].Task != "other" {
		t.Errorf("todos[1] = %+v, want {ID:2 Task:other Done:false}", todos[1])
	}
}
