package mimdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestIntrospect_ListTables verifies that ListTables sends a GET to the correct
// path and deserializes the envelope response into []TableSummary.
func TestIntrospect_ListTables(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/introspect/test-ref/tables" {
			t.Errorf("path = %q, want /v1/introspect/test-ref/tables", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"name":         "users",
					"schema":       "public",
					"row_estimate": 1500,
					"size_bytes":   65536,
					"comment":      "Application users",
				},
				{
					"name":         "posts",
					"schema":       "public",
					"row_estimate": 12000,
					"size_bytes":   524288,
					"comment":      nil,
				},
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
	tables, err := client.Introspect().ListTables(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tables) != 2 {
		t.Fatalf("len(tables) = %d, want 2", len(tables))
	}
	if tables[0].Name != "users" {
		t.Errorf("tables[0].Name = %q, want %q", tables[0].Name, "users")
	}
	if tables[0].Schema != "public" {
		t.Errorf("tables[0].Schema = %q, want %q", tables[0].Schema, "public")
	}
	if tables[0].RowEstimate != 1500 {
		t.Errorf("tables[0].RowEstimate = %d, want 1500", tables[0].RowEstimate)
	}
	if tables[0].SizeBytes != 65536 {
		t.Errorf("tables[0].SizeBytes = %d, want 65536", tables[0].SizeBytes)
	}
	if tables[0].Comment == nil || *tables[0].Comment != "Application users" {
		t.Errorf("tables[0].Comment = %v, want %q", tables[0].Comment, "Application users")
	}
	if tables[1].Name != "posts" {
		t.Errorf("tables[1].Name = %q, want %q", tables[1].Name, "posts")
	}
	if tables[1].Comment != nil {
		t.Errorf("tables[1].Comment = %v, want nil", tables[1].Comment)
	}
}

// TestIntrospect_GetTable verifies that GetTable sends a GET to the correct
// path with the table name interpolated and returns the full *TableDetail
// including columns, indexes, and foreign keys.
func TestIntrospect_GetTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/introspect/test-ref/tables/users" {
			t.Errorf("path = %q, want /v1/introspect/test-ref/tables/users", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"name":         "users",
				"schema":       "public",
				"row_estimate": 1500,
				"size_bytes":   65536,
				"comment":      "Application users",
				"primary_key":  []string{"id"},
				"columns": []map[string]any{
					{
						"name":             "id",
						"type":             "uuid",
						"nullable":         false,
						"default_value":    "gen_random_uuid()",
						"is_primary_key":   true,
						"comment":          nil,
						"ordinal_position": 1,
					},
					{
						"name":             "email",
						"type":             "text",
						"nullable":         false,
						"default_value":    nil,
						"is_primary_key":   false,
						"comment":          "User email address",
						"ordinal_position": 2,
					},
					{
						"name":             "org_id",
						"type":             "uuid",
						"nullable":         true,
						"default_value":    nil,
						"is_primary_key":   false,
						"comment":          nil,
						"ordinal_position": 3,
					},
				},
				"foreign_keys": []map[string]any{
					{
						"name":            "fk_users_org",
						"columns":         []string{"org_id"},
						"foreign_schema":  "public",
						"foreign_table":   "organizations",
						"foreign_columns": []string{"id"},
					},
				},
				"indexes": []map[string]any{
					{
						"name":       "users_pkey",
						"columns":    []string{"id"},
						"is_unique":  true,
						"is_primary": true,
						"type":       "btree",
					},
					{
						"name":       "users_email_idx",
						"columns":    []string{"email"},
						"is_unique":  true,
						"is_primary": false,
						"type":       "btree",
					},
				},
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-detail"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	detail, err := client.Introspect().GetTable(context.Background(), "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if detail == nil {
		t.Fatal("expected non-nil detail")
	}
	if detail.Name != "users" {
		t.Errorf("detail.Name = %q, want %q", detail.Name, "users")
	}
	if detail.Schema != "public" {
		t.Errorf("detail.Schema = %q, want %q", detail.Schema, "public")
	}

	// Verify columns.
	if len(detail.Columns) != 3 {
		t.Fatalf("len(Columns) = %d, want 3", len(detail.Columns))
	}
	if detail.Columns[0].Name != "id" {
		t.Errorf("Columns[0].Name = %q, want %q", detail.Columns[0].Name, "id")
	}
	if detail.Columns[0].Type != "uuid" {
		t.Errorf("Columns[0].Type = %q, want %q", detail.Columns[0].Type, "uuid")
	}
	if !detail.Columns[0].IsPrimaryKey {
		t.Errorf("Columns[0].IsPrimaryKey = false, want true")
	}
	if detail.Columns[1].Name != "email" {
		t.Errorf("Columns[1].Name = %q, want %q", detail.Columns[1].Name, "email")
	}
	if detail.Columns[1].Comment == nil || *detail.Columns[1].Comment != "User email address" {
		t.Errorf("Columns[1].Comment = %v, want %q", detail.Columns[1].Comment, "User email address")
	}

	// Verify foreign keys.
	if len(detail.ForeignKeys) != 1 {
		t.Fatalf("len(ForeignKeys) = %d, want 1", len(detail.ForeignKeys))
	}
	fk := detail.ForeignKeys[0]
	if fk.Name != "fk_users_org" {
		t.Errorf("ForeignKeys[0].Name = %q, want %q", fk.Name, "fk_users_org")
	}
	if fk.ForeignTable != "organizations" {
		t.Errorf("ForeignKeys[0].ForeignTable = %q, want %q", fk.ForeignTable, "organizations")
	}
	if len(fk.Columns) != 1 || fk.Columns[0] != "org_id" {
		t.Errorf("ForeignKeys[0].Columns = %v, want [org_id]", fk.Columns)
	}
	if len(fk.ForeignColumns) != 1 || fk.ForeignColumns[0] != "id" {
		t.Errorf("ForeignKeys[0].ForeignColumns = %v, want [id]", fk.ForeignColumns)
	}

	// Verify indexes.
	if len(detail.Indexes) != 2 {
		t.Fatalf("len(Indexes) = %d, want 2", len(detail.Indexes))
	}
	if detail.Indexes[0].Name != "users_pkey" {
		t.Errorf("Indexes[0].Name = %q, want %q", detail.Indexes[0].Name, "users_pkey")
	}
	if !detail.Indexes[0].IsPrimary {
		t.Errorf("Indexes[0].IsPrimary = false, want true")
	}
	if !detail.Indexes[0].IsUnique {
		t.Errorf("Indexes[0].IsUnique = false, want true")
	}
	if detail.Indexes[1].Name != "users_email_idx" {
		t.Errorf("Indexes[1].Name = %q, want %q", detail.Indexes[1].Name, "users_email_idx")
	}
	if detail.Indexes[1].IsPrimary {
		t.Errorf("Indexes[1].IsPrimary = true, want false")
	}

	// Verify primary key.
	if len(detail.PrimaryKey) != 1 || detail.PrimaryKey[0] != "id" {
		t.Errorf("PrimaryKey = %v, want [id]", detail.PrimaryKey)
	}
}

// TestIntrospect_RequiresProjectRef verifies that all introspection methods
// return an error when the client is configured without a ProjectRef.
func TestIntrospect_RequiresProjectRef(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		AdminSecret: "admin-secret",
	})
	ctx := context.Background()

	t.Run("ListTables", func(t *testing.T) {
		_, err := client.Introspect().ListTables(ctx)
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("GetTable", func(t *testing.T) {
		_, err := client.Introspect().GetTable(ctx, "users")
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})
}
