package mimdb

import (
	"context"
	"fmt"
	"net/http"
)

// VectorClient provides access to the MimDB vector similarity search API for
// managing vector-enabled tables, performing nearest-neighbor searches, and
// building HNSW indexes.
//
// Obtain a VectorClient via [Client.Vectors]. All operations require a
// ProjectRef to be configured on the parent Client.
type VectorClient struct {
	client *Client
}

// CreateVectorTableRequest holds the parameters for creating a new
// vector-enabled table.
type CreateVectorTableRequest struct {
	// Name is the table name.
	Name string `json:"name"`

	// Dimensions is the number of dimensions for vectors stored in the table.
	Dimensions int `json:"dimensions"`

	// Metric is the distance metric used for similarity searches. Valid values
	// are "cosine", "l2", and "inner_product". If empty, the server default
	// is used.
	Metric string `json:"metric,omitempty"`
}

// DeleteVectorTableOptions controls the behavior of a vector table deletion.
type DeleteVectorTableOptions struct {
	// Confirm must match the table name to authorize deletion.
	Confirm string

	// Cascade indicates whether dependent objects should also be dropped.
	Cascade bool
}

// VectorSearchRequest holds the parameters for a vector similarity search.
type VectorSearchRequest struct {
	// Vector is the query vector to search against.
	Vector []float32 `json:"vector"`

	// Limit controls the maximum number of results returned. If zero, the
	// server default is used.
	Limit int `json:"limit,omitempty"`
}

// CreateVectorIndexRequest holds optional tuning parameters for creating an
// HNSW index on a vector table.
type CreateVectorIndexRequest struct {
	// M is the maximum number of connections per node in the HNSW graph. If
	// zero, the server default is used.
	M int `json:"m,omitempty"`

	// EfConstruction controls the index build-time accuracy/speed trade-off.
	// If zero, the server default is used.
	EfConstruction int `json:"ef_construction,omitempty"`
}

// vectorBasePath returns the URL prefix for vector endpoints scoped to the
// client's project ref.
func (v *VectorClient) vectorBasePath() string {
	return fmt.Sprintf("/v1/vectors/%s", v.client.projectRef)
}

// CreateTable creates a new vector-enabled table in the project database.
//
//	err := client.Vectors().CreateTable(ctx, mimdb.CreateVectorTableRequest{
//	    Name:       "embeddings",
//	    Dimensions: 1536,
//	    Metric:     "cosine",
//	})
func (v *VectorClient) CreateTable(ctx context.Context, req CreateVectorTableRequest) error {
	if err := v.client.requireProjectRef(); err != nil {
		return err
	}
	path := fmt.Sprintf("%s/tables", v.vectorBasePath())
	err := v.client.transport.Do(ctx, http.MethodPost, path, req, nil)
	return wrapTransportError(err)
}

// ListTables retrieves all vector-enabled tables in the project database.
//
//	tables, err := client.Vectors().ListTables(ctx)
func (v *VectorClient) ListTables(ctx context.Context) ([]VectorTable, error) {
	if err := v.client.requireProjectRef(); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/tables", v.vectorBasePath())
	var tables []VectorTable
	err := v.client.transport.Do(ctx, http.MethodGet, path, nil, &tables)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return tables, nil
}

// DeleteTable deletes a vector-enabled table. The opts.Confirm field must
// match the table name, and opts.Cascade controls whether dependent objects
// are also dropped.
//
//	err := client.Vectors().DeleteTable(ctx, "embeddings", mimdb.DeleteVectorTableOptions{
//	    Confirm: "embeddings",
//	    Cascade: true,
//	})
func (v *VectorClient) DeleteTable(ctx context.Context, table string, opts DeleteVectorTableOptions) error {
	if err := v.client.requireProjectRef(); err != nil {
		return err
	}
	path := fmt.Sprintf("%s/tables/%s?confirm=%s", v.vectorBasePath(), table, opts.Confirm)
	if opts.Cascade {
		path += "&cascade=true"
	}
	err := v.client.transport.Do(ctx, http.MethodDelete, path, nil, nil)
	return wrapTransportError(err)
}

// Search performs a vector similarity search on the given table and returns
// the matching rows as untyped maps.
//
//	results, err := client.Vectors().Search(ctx, "embeddings", mimdb.VectorSearchRequest{
//	    Vector: queryVec,
//	    Limit:  10,
//	})
func (v *VectorClient) Search(ctx context.Context, table string, req VectorSearchRequest) ([]map[string]any, error) {
	if err := v.client.requireProjectRef(); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/%s/search", v.vectorBasePath(), table)
	var results []map[string]any
	err := v.client.transport.Do(ctx, http.MethodPost, path, req, &results)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return results, nil
}

// CreateIndex builds an HNSW index on the given vector table. The optional
// tuning parameters M and EfConstruction control the index quality vs. build
// speed trade-off.
//
//	err := client.Vectors().CreateIndex(ctx, "embeddings", mimdb.CreateVectorIndexRequest{
//	    M:              16,
//	    EfConstruction: 64,
//	})
func (v *VectorClient) CreateIndex(ctx context.Context, table string, req CreateVectorIndexRequest) error {
	if err := v.client.requireProjectRef(); err != nil {
		return err
	}
	path := fmt.Sprintf("%s/%s/index", v.vectorBasePath(), table)
	err := v.client.transport.Do(ctx, http.MethodPost, path, req, nil)
	return wrapTransportError(err)
}
