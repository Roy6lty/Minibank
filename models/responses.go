package models

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type APIResponse struct {
	Success bool      `json:"success"`
	Message string    `json:"message,omitempty"`
	Data    any       `json:"data,omitempty"`
	Error   *APIError `json:"error,omitempty"`
}
