package httpapi

import (
	"encoding/json"
	"net/http"
)

type apiError struct {
	Code    int    `json:"code"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, errType, message string) {
	WriteJSON(w, status, apiError{Code: status, Type: errType, Message: message})
}
