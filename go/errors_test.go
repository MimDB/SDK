package mimdb

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	err := &APIError{Code: "AUTH-0001", Message: "invalid token", HTTPStatus: 401}
	want := "AUTH-0001: invalid token"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_ErrorsAs(t *testing.T) {
	var wrapped error = &APIError{Code: "TEST-0001", Message: "test"}
	var apiErr *APIError
	if !errors.As(wrapped, &apiErr) {
		t.Fatal("errors.As should match *APIError")
	}
	if apiErr.Code != "TEST-0001" {
		t.Errorf("Code = %q, want TEST-0001", apiErr.Code)
	}
}

func TestAPIError_JSON(t *testing.T) {
	raw := `{"code":"STOR-0002","message":"bucket not found","detail":"Bucket 'avatars' not found"}`
	var apiErr APIError
	if err := json.Unmarshal([]byte(raw), &apiErr); err != nil {
		t.Fatal(err)
	}
	if apiErr.Code != "STOR-0002" || apiErr.Detail != "Bucket 'avatars' not found" {
		t.Errorf("unexpected parse: %+v", apiErr)
	}
}
