package mimdb

import (
	"context"
	"fmt"
	"net/http"
)

// CronClient provides access to the MimDB cron job scheduling API for creating,
// updating, and monitoring scheduled database commands.
//
// Obtain a CronClient via [Client.Cron]. All operations require a ProjectRef
// to be configured on the parent Client.
type CronClient struct {
	client *Client
}

// CreateCronJobRequest holds the parameters for creating a new cron job.
type CreateCronJobRequest struct {
	// Name is the human-readable name for the job.
	Name string `json:"name"`

	// Schedule is the cron expression (e.g. "*/5 * * * *").
	Schedule string `json:"schedule"`

	// Command is the SQL command to execute on each trigger.
	Command string `json:"command"`
}

// UpdateCronJobRequest holds the parameters for updating an existing cron job.
// Only non-zero fields are sent to the server.
type UpdateCronJobRequest struct {
	// Schedule is the updated cron expression.
	Schedule string `json:"schedule,omitempty"`

	// Command is the updated SQL command.
	Command string `json:"command,omitempty"`

	// Active controls whether the job is enabled or paused. A nil value leaves
	// the current state unchanged.
	Active *bool `json:"active,omitempty"`
}

// cronJobsResponse is the internal envelope wrapper for the ListJobs response.
type cronJobsResponse struct {
	Jobs       []CronJob `json:"jobs"`
	Total      int       `json:"total"`
	MaxAllowed int       `json:"max_allowed"`
}

// cronJobHistoryResponse is the internal envelope wrapper for the GetJobHistory
// response.
type cronJobHistoryResponse struct {
	History []CronJobRun `json:"history"`
	Total   int          `json:"total"`
}

// cronBasePath returns the URL prefix for cron endpoints scoped to the
// client's project ref.
func (c *CronClient) cronBasePath() string {
	return fmt.Sprintf("/v1/cron/%s", c.client.projectRef)
}

// CreateJob creates a new scheduled cron job in the project database.
//
//	job, err := client.Cron().CreateJob(ctx, mimdb.CreateCronJobRequest{
//	    Name:     "vacuum-daily",
//	    Schedule: "0 3 * * *",
//	    Command:  "VACUUM ANALYZE",
//	})
func (c *CronClient) CreateJob(ctx context.Context, req CreateCronJobRequest) (*CronJob, error) {
	if err := c.client.requireProjectRef(); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/jobs", c.cronBasePath())
	var job CronJob
	err := c.client.transport.Do(ctx, http.MethodPost, path, req, &job)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &job, nil
}

// ListJobs retrieves all cron jobs configured in the project database.
//
//	jobs, err := client.Cron().ListJobs(ctx)
func (c *CronClient) ListJobs(ctx context.Context) ([]CronJob, error) {
	if err := c.client.requireProjectRef(); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/jobs", c.cronBasePath())
	var resp cronJobsResponse
	err := c.client.transport.Do(ctx, http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return resp.Jobs, nil
}

// GetJob retrieves a single cron job by its ID.
//
//	job, err := client.Cron().GetJob(ctx, 42)
func (c *CronClient) GetJob(ctx context.Context, jobID int64) (*CronJob, error) {
	if err := c.client.requireProjectRef(); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/jobs/%d", c.cronBasePath(), jobID)
	var job CronJob
	err := c.client.transport.Do(ctx, http.MethodGet, path, nil, &job)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &job, nil
}

// UpdateJob updates an existing cron job. Only non-zero fields in the request
// are applied.
//
//	active := false
//	job, err := client.Cron().UpdateJob(ctx, 42, mimdb.UpdateCronJobRequest{
//	    Active: &active,
//	})
func (c *CronClient) UpdateJob(ctx context.Context, jobID int64, req UpdateCronJobRequest) (*CronJob, error) {
	if err := c.client.requireProjectRef(); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/jobs/%d", c.cronBasePath(), jobID)
	var job CronJob
	err := c.client.transport.Do(ctx, http.MethodPatch, path, req, &job)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &job, nil
}

// DeleteJob removes a cron job by its ID.
//
//	err := client.Cron().DeleteJob(ctx, 42)
func (c *CronClient) DeleteJob(ctx context.Context, jobID int64) error {
	if err := c.client.requireProjectRef(); err != nil {
		return err
	}
	path := fmt.Sprintf("%s/jobs/%d", c.cronBasePath(), jobID)
	err := c.client.transport.Do(ctx, http.MethodDelete, path, nil, nil)
	return wrapTransportError(err)
}

// GetJobHistory retrieves the execution history for a cron job.
//
//	runs, err := client.Cron().GetJobHistory(ctx, 42)
func (c *CronClient) GetJobHistory(ctx context.Context, jobID int64) ([]CronJobRun, error) {
	if err := c.client.requireProjectRef(); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/jobs/%d/history", c.cronBasePath(), jobID)
	var resp cronJobHistoryResponse
	err := c.client.transport.Do(ctx, http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return resp.History, nil
}
