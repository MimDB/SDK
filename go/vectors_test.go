package mimdb

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestVectors_CreateTable verifies that CreateTable sends a POST to the correct
// path with the expected JSON body.
func TestVectors_CreateTable(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/vectors/test-ref/tables" {
			t.Errorf("path = %q, want /v1/vectors/test-ref/tables", r.URL.Path)
		}

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	err := client.Vectors().CreateTable(context.Background(), CreateVectorTableRequest{
		Name:       "embeddings",
		Dimensions: 1536,
		Metric:     "cosine",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if body["name"] != "embeddings" {
		t.Errorf("body name = %v, want %q", body["name"], "embeddings")
	}
	if body["dimensions"] != float64(1536) {
		t.Errorf("body dimensions = %v, want 1536", body["dimensions"])
	}
	if body["metric"] != "cosine" {
		t.Errorf("body metric = %v, want %q", body["metric"], "cosine")
	}
}

// TestVectors_ListTables verifies that ListTables sends a GET to the correct
// path and deserializes the envelope response into []VectorTable.
func TestVectors_ListTables(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/vectors/test-ref/tables" {
			t.Errorf("path = %q, want /v1/vectors/test-ref/tables", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"name": "embeddings", "dimensions": 1536},
				{"name": "images", "dimensions": 512},
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-list"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	tables, err := client.Vectors().ListTables(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tables) != 2 {
		t.Fatalf("len(tables) = %d, want 2", len(tables))
	}
	if tables[0].Name != "embeddings" {
		t.Errorf("tables[0].Name = %q, want %q", tables[0].Name, "embeddings")
	}
	if tables[0].Dimensions != 1536 {
		t.Errorf("tables[0].Dimensions = %d, want 1536", tables[0].Dimensions)
	}
	if tables[1].Name != "images" {
		t.Errorf("tables[1].Name = %q, want %q", tables[1].Name, "images")
	}
	if tables[1].Dimensions != 512 {
		t.Errorf("tables[1].Dimensions = %d, want 512", tables[1].Dimensions)
	}
}

// TestVectors_DeleteTable verifies that DeleteTable sends a DELETE with the
// confirm and cascade query parameters.
func TestVectors_DeleteTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/v1/vectors/test-ref/tables/embeddings" {
			t.Errorf("path = %q, want /v1/vectors/test-ref/tables/embeddings", r.URL.Path)
		}
		if r.URL.Query().Get("confirm") != "embeddings" {
			t.Errorf("confirm = %q, want %q", r.URL.Query().Get("confirm"), "embeddings")
		}
		if r.URL.Query().Get("cascade") != "true" {
			t.Errorf("cascade = %q, want %q", r.URL.Query().Get("cascade"), "true")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	err := client.Vectors().DeleteTable(context.Background(), "embeddings", DeleteVectorTableOptions{
		Confirm: "embeddings",
		Cascade: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestVectors_Search verifies that Search sends a POST with float32 vectors
// and returns the results as []map[string]any.
func TestVectors_Search(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/vectors/test-ref/embeddings/search" {
			t.Errorf("path = %q, want /v1/vectors/test-ref/embeddings/search", r.URL.Path)
		}

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "row-1", "similarity": 0.95, "content": "hello"},
				{"id": "row-2", "similarity": 0.87, "content": "world"},
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-search"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	results, err := client.Vectors().Search(context.Background(), "embeddings", VectorSearchRequest{
		Vector: []float32{0.1, 0.2, 0.3},
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the request body contains the float32 vector.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	vecArr, ok := body["vector"].([]any)
	if !ok {
		t.Fatalf("body vector is not an array: %T", body["vector"])
	}
	if len(vecArr) != 3 {
		t.Fatalf("body vector length = %d, want 3", len(vecArr))
	}

	// Verify the response.
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0]["id"] != "row-1" {
		t.Errorf("results[0][id] = %v, want %q", results[0]["id"], "row-1")
	}
	if results[1]["id"] != "row-2" {
		t.Errorf("results[1][id] = %v, want %q", results[1]["id"], "row-2")
	}
}

// TestVectors_CreateIndex verifies that CreateIndex sends a POST with the
// HNSW tuning parameters.
func TestVectors_CreateIndex(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/vectors/test-ref/embeddings/index" {
			t.Errorf("path = %q, want /v1/vectors/test-ref/embeddings/index", r.URL.Path)
		}

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	err := client.Vectors().CreateIndex(context.Background(), "embeddings", CreateVectorIndexRequest{
		M:              16,
		EfConstruction: 64,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if body["m"] != float64(16) {
		t.Errorf("body m = %v, want 16", body["m"])
	}
	if body["ef_construction"] != float64(64) {
		t.Errorf("body ef_construction = %v, want 64", body["ef_construction"])
	}
}

// TestVectors_RequiresProjectRef verifies that all vector methods return an
// error when the client is configured without a ProjectRef.
func TestVectors_RequiresProjectRef(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		AdminSecret: "admin-secret",
	})
	ctx := context.Background()

	t.Run("CreateTable", func(t *testing.T) {
		err := client.Vectors().CreateTable(ctx, CreateVectorTableRequest{Name: "t", Dimensions: 3})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("ListTables", func(t *testing.T) {
		_, err := client.Vectors().ListTables(ctx)
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("DeleteTable", func(t *testing.T) {
		err := client.Vectors().DeleteTable(ctx, "t", DeleteVectorTableOptions{Confirm: "t"})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("Search", func(t *testing.T) {
		_, err := client.Vectors().Search(ctx, "t", VectorSearchRequest{Vector: []float32{0.1}})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("CreateIndex", func(t *testing.T) {
		err := client.Vectors().CreateIndex(ctx, "t", CreateVectorIndexRequest{})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})
}
