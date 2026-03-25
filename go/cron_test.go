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

// TestCron_CreateJob verifies that CreateJob sends a POST to the correct path
// with the expected JSON body and returns the created *CronJob.
func TestCron_CreateJob(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/cron/test-ref/jobs" {
			t.Errorf("path = %q, want /v1/cron/test-ref/jobs", r.URL.Path)
		}

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":       42,
				"name":     "vacuum-daily",
				"schedule": "0 3 * * *",
				"command":  "VACUUM ANALYZE",
				"active":   true,
				"database": "postgres",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-create"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	job, err := client.Cron().CreateJob(context.Background(), CreateCronJobRequest{
		Name:     "vacuum-daily",
		Schedule: "0 3 * * *",
		Command:  "VACUUM ANALYZE",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request body.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if body["name"] != "vacuum-daily" {
		t.Errorf("body name = %v, want %q", body["name"], "vacuum-daily")
	}
	if body["schedule"] != "0 3 * * *" {
		t.Errorf("body schedule = %v, want %q", body["schedule"], "0 3 * * *")
	}
	if body["command"] != "VACUUM ANALYZE" {
		t.Errorf("body command = %v, want %q", body["command"], "VACUUM ANALYZE")
	}

	// Verify response.
	if job == nil {
		t.Fatal("expected non-nil job")
	}
	if job.ID != 42 {
		t.Errorf("job.ID = %d, want 42", job.ID)
	}
	if job.Name != "vacuum-daily" {
		t.Errorf("job.Name = %q, want %q", job.Name, "vacuum-daily")
	}
	if job.Schedule != "0 3 * * *" {
		t.Errorf("job.Schedule = %q, want %q", job.Schedule, "0 3 * * *")
	}
	if !job.Active {
		t.Errorf("job.Active = false, want true")
	}
}

// TestCron_ListJobs verifies that ListJobs sends a GET to the correct path and
// extracts the jobs array from the wrapper response.
func TestCron_ListJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/cron/test-ref/jobs" {
			t.Errorf("path = %q, want /v1/cron/test-ref/jobs", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"jobs": []map[string]any{
					{"id": 1, "name": "job-a", "schedule": "* * * * *", "command": "SELECT 1", "active": true},
					{"id": 2, "name": "job-b", "schedule": "0 * * * *", "command": "SELECT 2", "active": false},
				},
				"total":       2,
				"max_allowed": 10,
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
	jobs, err := client.Cron().ListJobs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("len(jobs) = %d, want 2", len(jobs))
	}
	if jobs[0].ID != 1 {
		t.Errorf("jobs[0].ID = %d, want 1", jobs[0].ID)
	}
	if jobs[0].Name != "job-a" {
		t.Errorf("jobs[0].Name = %q, want %q", jobs[0].Name, "job-a")
	}
	if jobs[1].ID != 2 {
		t.Errorf("jobs[1].ID = %d, want 2", jobs[1].ID)
	}
	if jobs[1].Active {
		t.Errorf("jobs[1].Active = true, want false")
	}
}

// TestCron_GetJob verifies that GetJob sends a GET to the correct path with
// the job ID interpolated and returns the *CronJob.
func TestCron_GetJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/cron/test-ref/jobs/42" {
			t.Errorf("path = %q, want /v1/cron/test-ref/jobs/42", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":       42,
				"name":     "vacuum-daily",
				"schedule": "0 3 * * *",
				"command":  "VACUUM ANALYZE",
				"active":   true,
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-get"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	job, err := client.Cron().GetJob(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job == nil {
		t.Fatal("expected non-nil job")
	}
	if job.ID != 42 {
		t.Errorf("job.ID = %d, want 42", job.ID)
	}
	if job.Name != "vacuum-daily" {
		t.Errorf("job.Name = %q, want %q", job.Name, "vacuum-daily")
	}
}

// TestCron_UpdateJob verifies that UpdateJob sends a PATCH to the correct path
// with the expected JSON body and returns the updated *CronJob.
func TestCron_UpdateJob(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %q, want PATCH", r.Method)
		}
		if r.URL.Path != "/v1/cron/test-ref/jobs/42" {
			t.Errorf("path = %q, want /v1/cron/test-ref/jobs/42", r.URL.Path)
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
				"id":       42,
				"name":     "vacuum-daily",
				"schedule": "0 6 * * *",
				"command":  "VACUUM ANALYZE",
				"active":   false,
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-update"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	active := false
	job, err := client.Cron().UpdateJob(context.Background(), 42, UpdateCronJobRequest{
		Schedule: "0 6 * * *",
		Active:   &active,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request body.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if body["schedule"] != "0 6 * * *" {
		t.Errorf("body schedule = %v, want %q", body["schedule"], "0 6 * * *")
	}
	if body["active"] != false {
		t.Errorf("body active = %v, want false", body["active"])
	}

	// Verify response.
	if job == nil {
		t.Fatal("expected non-nil job")
	}
	if job.Schedule != "0 6 * * *" {
		t.Errorf("job.Schedule = %q, want %q", job.Schedule, "0 6 * * *")
	}
	if job.Active {
		t.Errorf("job.Active = true, want false")
	}
}

// TestCron_DeleteJob verifies that DeleteJob sends a DELETE to the correct path.
func TestCron_DeleteJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/v1/cron/test-ref/jobs/42" {
			t.Errorf("path = %q, want /v1/cron/test-ref/jobs/42", r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	err := client.Cron().DeleteJob(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCron_GetJobHistory verifies that GetJobHistory sends a GET to the correct
// path and extracts the history array from the wrapper response.
func TestCron_GetJobHistory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/cron/test-ref/jobs/42/history" {
			t.Errorf("path = %q, want /v1/cron/test-ref/jobs/42/history", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"history": []map[string]any{
					{
						"run_id":         1,
						"job_id":         42,
						"command":        "VACUUM ANALYZE",
						"status":         "succeeded",
						"return_message": "VACUUM",
						"start_time":     "2024-01-01T03:00:00Z",
						"end_time":       "2024-01-01T03:00:05Z",
					},
					{
						"run_id":         2,
						"job_id":         42,
						"command":        "VACUUM ANALYZE",
						"status":         "failed",
						"return_message": "ERROR: permission denied",
						"start_time":     "2024-01-02T03:00:00Z",
						"end_time":       "2024-01-02T03:00:01Z",
					},
				},
				"total": 2,
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-history"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	runs, err := client.Cron().GetJobHistory(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runs) != 2 {
		t.Fatalf("len(runs) = %d, want 2", len(runs))
	}
	if runs[0].RunID != 1 {
		t.Errorf("runs[0].RunID = %d, want 1", runs[0].RunID)
	}
	if runs[0].Status != "succeeded" {
		t.Errorf("runs[0].Status = %q, want %q", runs[0].Status, "succeeded")
	}
	if runs[1].RunID != 2 {
		t.Errorf("runs[1].RunID = %d, want 2", runs[1].RunID)
	}
	if runs[1].Status != "failed" {
		t.Errorf("runs[1].Status = %q, want %q", runs[1].Status, "failed")
	}
	if runs[1].ReturnMessage != "ERROR: permission denied" {
		t.Errorf("runs[1].ReturnMessage = %q, want %q", runs[1].ReturnMessage, "ERROR: permission denied")
	}
}

// TestCron_RequiresProjectRef verifies that all cron methods return an error
// when the client is configured without a ProjectRef.
func TestCron_RequiresProjectRef(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		AdminSecret: "admin-secret",
	})
	ctx := context.Background()

	t.Run("CreateJob", func(t *testing.T) {
		_, err := client.Cron().CreateJob(ctx, CreateCronJobRequest{Name: "j"})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("ListJobs", func(t *testing.T) {
		_, err := client.Cron().ListJobs(ctx)
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("GetJob", func(t *testing.T) {
		_, err := client.Cron().GetJob(ctx, 1)
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("UpdateJob", func(t *testing.T) {
		_, err := client.Cron().UpdateJob(ctx, 1, UpdateCronJobRequest{})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("DeleteJob", func(t *testing.T) {
		err := client.Cron().DeleteJob(ctx, 1)
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("GetJobHistory", func(t *testing.T) {
		_, err := client.Cron().GetJobHistory(ctx, 1)
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})
}

// TestCron_ErrorResponse verifies that API errors are properly wrapped as
// *APIError.
func TestCron_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": nil,
			"error": map[string]any{
				"code":    "CRON-0001",
				"message": "job not found",
				"detail":  "no cron job with id 999",
			},
			"meta": map[string]string{"request_id": "req-err"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "test-ref",
		APIKey:     "test-key",
	})
	_, err := client.Cron().GetJob(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "CRON-0001" {
		t.Errorf("apiErr.Code = %q, want %q", apiErr.Code, "CRON-0001")
	}
	if apiErr.Message != "job not found" {
		t.Errorf("apiErr.Message = %q, want %q", apiErr.Message, "job not found")
	}
}
