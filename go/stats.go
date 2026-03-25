package mimdb

import (
	"context"
	"fmt"
	"net/http"
)

// StatsClient provides access to the MimDB query statistics API for retrieving
// performance data from pg_stat_statements.
//
// Obtain a StatsClient via [Client.Stats]. All operations require a ProjectRef
// to be configured on the parent Client.
type StatsClient struct {
	client *Client
}

// QueryStatsOptions holds optional parameters for filtering and ordering query
// statistics.
type QueryStatsOptions struct {
	// OrderBy controls the sort order of returned queries. Valid values are
	// "total_time", "mean_time", "calls", and "rows".
	OrderBy string

	// Limit controls the maximum number of queries returned. If zero, the
	// server default is used.
	Limit int
}

// GetQueryStats retrieves query performance statistics from
// pg_stat_statements. An optional [QueryStatsOptions] can be provided to
// control ordering and limit.
//
//	stats, err := client.Stats().GetQueryStats(ctx, mimdb.QueryStatsOptions{
//	    OrderBy: "total_time",
//	    Limit:   20,
//	})
func (s *StatsClient) GetQueryStats(ctx context.Context, opts ...QueryStatsOptions) (*QueryStatsResponse, error) {
	if err := s.client.requireProjectRef(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/stats/%s/queries", s.client.projectRef)

	// Apply optional query parameters.
	if len(opts) > 0 {
		o := opts[0]
		sep := "?"
		if o.OrderBy != "" {
			path += fmt.Sprintf("%sorder_by=%s", sep, o.OrderBy)
			sep = "&"
		}
		if o.Limit > 0 {
			path += fmt.Sprintf("%slimit=%d", sep, o.Limit)
		}
	}

	var resp QueryStatsResponse
	err := s.client.transport.Do(ctx, http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &resp, nil
}
