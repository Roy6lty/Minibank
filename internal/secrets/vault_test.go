package secrets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoadVaultKV2(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/secret/data/minibank" {
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		if r.Header.Get("X-Vault-Token") != "test-token" {
			http.Error(w, "unexpected token", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"data":{"DB_PASSWORD":"db-password","REDIS_PASSWORD":"redis-password","ACCESS_SECRET":"access-secret","REFRESH_SECRET":"refresh-secret"},"metadata":{}}}`))
	}))
	defer server.Close()

	values, err := LoadVaultKV2(context.Background(), server.URL, "test-token", "minibank")
	if err != nil {
		t.Fatalf("LoadVaultKV2 returned an error: %v", err)
	}
	if values["DB_PASSWORD"] != "db-password" || values["REFRESH_SECRET"] != "refresh-secret" {
		t.Fatalf("unexpected Vault values: %#v", values)
	}
}
