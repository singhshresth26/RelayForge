package response

import (
	"encoding/json"
	"net/http"
	"time"
)

type APIResponse struct {
	Success   bool        `json:"success"`
	Data      any         `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	Timestamp string      `json:"timestamp"`
}

type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	resp := APIResponse{
		Success:   status >= 200 && status < 300,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func Error(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	resp := APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	_ = json.NewEncoder(w).Encode(resp)
}

