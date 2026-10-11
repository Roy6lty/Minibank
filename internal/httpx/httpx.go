package httpx

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

func DecodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(destination); err != nil {
		http.Error(w, "invalid JSON request body", http.StatusBadRequest)
		return false
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "request body must contain a single JSON value", http.StatusBadRequest)
		return false
	}

	return true
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("failed to write JSON response", "error", err)
	}
}
