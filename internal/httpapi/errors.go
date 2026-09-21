package httpapi

import (
	"encoding/json"
	"net/http"
)

// apiError is the JSON error body shape, matching the OpenAPI spec's
// ApiResponse schema (code, type, message) so error responses conform
// to the spec just as success responses do.
type apiError struct {
	Code    int    `json:"code"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

// WriteJSON writes v as a JSON response body with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes a spec-shaped error body with the given status,
// error type (a short machine-readable slug), and human-readable
// message.
func WriteError(w http.ResponseWriter, status int, errType, message string) {
	WriteJSON(w, status, apiError{Code: status, Type: errType, Message: message})
}
