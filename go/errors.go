package mimdb

import "fmt"

// APIError is a structured error returned by the MimDB API.
//
// It maps to the error object within the standard API envelope:
//
//	{"code": "AUTH-0001", "message": "invalid token", "detail": "..."}
//
// HTTPStatus and RequestID are populated from the HTTP response metadata
// and excluded from JSON serialization.
type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Detail     string `json:"detail,omitempty"`
	HTTPStatus int    `json:"-"`
	RequestID  string `json:"-"`
}

// Error implements the error interface. The format is "CODE: message" or
// "CODE: message (detail)" when a detail string is present.
func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Page is a generic paginated result for cursor-based list endpoints.
//
// T is the element type contained in each page. NextCursor is an opaque
// token that can be passed to the next request to fetch subsequent results.
type Page[T any] struct {
	Data       []T    `json:"data"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}
