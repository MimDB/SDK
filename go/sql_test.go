package mimdb

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSQL_Execute verifies that Execute sends a POST to the correct path with
// the query and returns the deserialized *SQLResult.
func TestSQL_Execute(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/sql/test-ref/execute" {
			t.Errorf("path = %q, want /v1/sql/test-ref/execute", r.URL.Path)
		}

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"columns": []map[string]any{
					{"name": "id", "type": "uuid"},
					{"name": "name", "type": "text"},
				},
				"rows": []map[string]any{
					{"id": "u-1", "name": "Alice"},
					{"id": "u-2", "name": "Bob"},
				},
				"row_count":         2,
				"truncated":         false,
				"execution_time_ms": 1.23,
				"command_tag":       "SELECT 2",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-sql"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	result, err := client.SQL().Execute(context.Background(), "SELECT id, name FROM users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request body.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if body["query"] != "SELECT id, name FROM users" {
		t.Errorf("body query = %v, want %q", body["query"], "SELECT id, name FROM users")
	}

	// Verify response.
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Columns) != 2 {
		t.Fatalf("len(Columns) = %d, want 2", len(result.Columns))
	}
	if result.Columns[0].Name != "id" {
		t.Errorf("Columns[0].Name = %q, want %q", result.Columns[0].Name, "id")
	}
	if result.Columns[0].Type != "uuid" {
		t.Errorf("Columns[0].Type = %q, want %q", result.Columns[0].Type, "uuid")
	}
	if result.Columns[1].Name != "name" {
		t.Errorf("Columns[1].Name = %q, want %q", result.Columns[1].Name, "name")
	}
	if len(result.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2", len(result.Rows))
	}
	if result.Rows[0]["id"] != "u-1" {
		t.Errorf("Rows[0][id] = %v, want %q", result.Rows[0]["id"], "u-1")
	}
	if result.Rows[1]["name"] != "Bob" {
		t.Errorf("Rows[1][name] = %v, want %q", result.Rows[1]["name"], "Bob")
	}
	if result.RowCount != 2 {
		t.Errorf("RowCount = %d, want 2", result.RowCount)
	}
	if result.Truncated {
		t.Errorf("Truncated = true, want false")
	}
	if result.ExecTimeMs != 1.23 {
		t.Errorf("ExecTimeMs = %f, want 1.23", result.ExecTimeMs)
	}
	if result.CommandTag != "SELECT 2" {
		t.Errorf("CommandTag = %q, want %q", result.CommandTag, "SELECT 2")
	}
}

// TestSQL_ExecuteWithParams verifies that Execute correctly sends positional
// parameters in the request body.
func TestSQL_ExecuteWithParams(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"columns":           []map[string]any{{"name": "id", "type": "uuid"}},
				"rows":              []map[string]any{{"id": "u-1"}},
				"row_count":         1,
				"truncated":         false,
				"execution_time_ms": 0.5,
				"command_tag":       "SELECT 1",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-sql-params"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	_, err := client.SQL().Execute(context.Background(), "SELECT * FROM users WHERE id = $1 AND active = $2", "u-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify params were sent.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	params, ok := body["params"].([]any)
	if !ok {
		t.Fatalf("body params is not an array: %T", body["params"])
	}
	if len(params) != 2 {
		t.Fatalf("len(params) = %d, want 2", len(params))
	}
	if params[0] != "u-1" {
		t.Errorf("params[0] = %v, want %q", params[0], "u-1")
	}
	if params[1] != true {
		t.Errorf("params[1] = %v, want true", params[1])
	}
}

// TestSQL_RequiresProjectRef verifies that Execute returns an error when the
// client is configured without a ProjectRef.
func TestSQL_RequiresProjectRef(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		AdminSecret: "admin-secret",
	})

	_, err := client.SQL().Execute(context.Background(), "SELECT 1")
	if err == nil {
		t.Fatal("expected error for missing ProjectRef")
	}
}

// TestSQL_ErrorResponse verifies that API errors are properly wrapped as
// *APIError.
func TestSQL_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": nil,
			"error": map[string]any{
				"code":    "SQL-0001",
				"message": "syntax error",
				"detail":  "at or near \"SELCT\"",
			},
			"meta": map[string]string{"request_id": "req-sql-err"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	_, err := client.SQL().Execute(context.Background(), "SELCT 1")
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "SQL-0001" {
		t.Errorf("apiErr.Code = %q, want %q", apiErr.Code, "SQL-0001")
	}
	if apiErr.Message != "syntax error" {
		t.Errorf("apiErr.Message = %q, want %q", apiErr.Message, "syntax error")
	}
}
