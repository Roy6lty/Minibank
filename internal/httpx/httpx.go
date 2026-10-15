package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/Roy6lty/minibank/internal/logging"
	"github.com/go-playground/validator/v10"
)

const MaxJSONBodySize int64 = 1 << 20

var requestValidator = validator.New()

func DecodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, MaxJSONBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeDecodeError(w, err)
		return false
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err != nil {
			writeDecodeError(w, err)
			return false
		}
		http.Error(w, "request body must contain a single JSON value", http.StatusBadRequest)
		return false
	}

	if err := requestValidator.Struct(destination); err != nil {
		http.Error(w, "request body failed validation", http.StatusBadRequest)
		return false
	}

	return true
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	http.Error(w, "invalid JSON request body", http.StatusBadRequest)
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("failed to write JSON response", logging.Error(err))
	}
}
