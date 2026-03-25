package mimdb

import (
	"context"
	"fmt"
	"net/http"
)

// IntrospectClient provides access to the MimDB schema introspection API for
// listing tables and inspecting column, index, and foreign-key metadata.
//
// Obtain an IntrospectClient via [Client.Introspect]. All operations require a
// ProjectRef to be configured on the parent Client.
type IntrospectClient struct {
	client *Client
}

// introspectBasePath returns the URL prefix for introspection endpoints scoped
// to the client's project ref.
func (i *IntrospectClient) introspectBasePath() string {
	return fmt.Sprintf("/v1/introspect/%s", i.client.projectRef)
}

// ListTables retrieves a summary of all tables in the project database,
// including row estimates and disk size.
//
//	tables, err := client.Introspect().ListTables(ctx)
func (i *IntrospectClient) ListTables(ctx context.Context) ([]TableSummary, error) {
	if err := i.client.requireProjectRef(); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/tables", i.introspectBasePath())
	var tables []TableSummary
	err := i.client.transport.Do(ctx, http.MethodGet, path, nil, &tables)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return tables, nil
}

// GetTable retrieves full introspection data for a single table, including
// columns, primary key, foreign keys, and indexes.
//
//	detail, err := client.Introspect().GetTable(ctx, "users")
func (i *IntrospectClient) GetTable(ctx context.Context, table string) (*TableDetail, error) {
	if err := i.client.requireProjectRef(); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/tables/%s", i.introspectBasePath(), table)
	var detail TableDetail
	err := i.client.transport.Do(ctx, http.MethodGet, path, nil, &detail)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &detail, nil
}
