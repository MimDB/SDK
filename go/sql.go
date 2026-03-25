package mimdb

import (
	"context"
	"fmt"
	"net/http"
)

// SQLClient provides access to the MimDB SQL execution API for running raw
// SQL queries against the project database.
//
// Obtain a SQLClient via [Client.SQL]. All operations require a ProjectRef
// to be configured on the parent Client.
type SQLClient struct {
	client *Client
}

// sqlExecuteRequest is the internal request body for the SQL execute endpoint.
type sqlExecuteRequest struct {
	Query  string `json:"query"`
	Params []any  `json:"params,omitempty"`
}

// Execute runs a SQL query against the project database and returns the result
// set. Parameters are passed as positional placeholders ($1, $2, ...) and are
// safely interpolated server-side.
//
//	result, err := client.SQL().Execute(ctx, "SELECT * FROM users WHERE id = $1", "user-123")
func (s *SQLClient) Execute(ctx context.Context, query string, params ...any) (*SQLResult, error) {
	if err := s.client.requireProjectRef(); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/v1/sql/%s/execute", s.client.projectRef)
	body := sqlExecuteRequest{
		Query:  query,
		Params: params,
	}
	var result SQLResult
	err := s.client.transport.Do(ctx, http.MethodPost, path, body, &result)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &result, nil
}
