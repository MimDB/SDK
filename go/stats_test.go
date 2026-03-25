package mimdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestStats_GetQueryStats verifies that GetQueryStats sends a GET to the
// correct path and deserializes the envelope response into *QueryStatsResponse.
func TestStats_GetQueryStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/stats/test-ref/queries" {
			t.Errorf("path = %q, want /v1/stats/test-ref/queries", r.URL.Path)
		}
		// No query params for default call.
		if r.URL.RawQuery != "" {
			t.Errorf("query = %q, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"queries": []map[string]any{
					{
						"queryid":            12345,
						"query":              "SELECT * FROM users",
						"calls":              100,
						"mean_exec_time_ms":  1.5,
						"total_exec_time_ms": 150.0,
					},
					{
						"queryid":            67890,
						"query":              "INSERT INTO logs VALUES ($1)",
						"calls":              5000,
						"mean_exec_time_ms":  0.3,
						"total_exec_time_ms": 1500.0,
					},
				},
				"total_queries": 2,
				"stats_reset":   "2024-01-01T00:00:00Z",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-stats"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	resp, err := client.Stats().GetQueryStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Queries) != 2 {
		t.Fatalf("len(Queries) = %d, want 2", len(resp.Queries))
	}
	if resp.Queries[0].QueryID != 12345 {
		t.Errorf("Queries[0].QueryID = %d, want 12345", resp.Queries[0].QueryID)
	}
	if resp.Queries[0].Query != "SELECT * FROM users" {
		t.Errorf("Queries[0].Query = %q, want %q", resp.Queries[0].Query, "SELECT * FROM users")
	}
	if resp.Queries[0].Calls != 100 {
		t.Errorf("Queries[0].Calls = %d, want 100", resp.Queries[0].Calls)
	}
	if resp.Queries[0].MeanExecTimeMs != 1.5 {
		t.Errorf("Queries[0].MeanExecTimeMs = %f, want 1.5", resp.Queries[0].MeanExecTimeMs)
	}
	if resp.Queries[0].TotalExecTimeMs != 150.0 {
		t.Errorf("Queries[0].TotalExecTimeMs = %f, want 150.0", resp.Queries[0].TotalExecTimeMs)
	}
	if resp.Queries[1].QueryID != 67890 {
		t.Errorf("Queries[1].QueryID = %d, want 67890", resp.Queries[1].QueryID)
	}
	if resp.Queries[1].Calls != 5000 {
		t.Errorf("Queries[1].Calls = %d, want 5000", resp.Queries[1].Calls)
	}
	if resp.TotalQueries != 2 {
		t.Errorf("TotalQueries = %d, want 2", resp.TotalQueries)
	}
	if resp.StatsReset == nil {
		t.Error("StatsReset should not be nil")
	} else if *resp.StatsReset != "2024-01-01T00:00:00Z" {
		t.Errorf("StatsReset = %q, want %q", *resp.StatsReset, "2024-01-01T00:00:00Z")
	}
}

// TestStats_GetQueryStatsWithOptions verifies that GetQueryStats sends the
// correct query parameters when options are provided.
func TestStats_GetQueryStatsWithOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/stats/test-ref/queries" {
			t.Errorf("path = %q, want /v1/stats/test-ref/queries", r.URL.Path)
		}
		if r.URL.Query().Get("order_by") != "total_time" {
			t.Errorf("order_by = %q, want %q", r.URL.Query().Get("order_by"), "total_time")
		}
		if r.URL.Query().Get("limit") != "20" {
			t.Errorf("limit = %q, want %q", r.URL.Query().Get("limit"), "20")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"queries":       []map[string]any{},
				"total_queries": 0,
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-stats-opts"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	resp, err := client.Stats().GetQueryStats(context.Background(), QueryStatsOptions{
		OrderBy: "total_time",
		Limit:   20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Queries) != 0 {
		t.Errorf("len(Queries) = %d, want 0", len(resp.Queries))
	}
}

// TestStats_RequiresProjectRef verifies that GetQueryStats returns an error
// when the client is configured without a ProjectRef.
func TestStats_RequiresProjectRef(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		AdminSecret: "admin-secret",
	})

	_, err := client.Stats().GetQueryStats(context.Background())
	if err == nil {
		t.Fatal("expected error for missing ProjectRef")
	}
}
