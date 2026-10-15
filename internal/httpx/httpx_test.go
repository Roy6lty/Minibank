package httpx_test

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Roy6lty/minibank/internal/httpx"
	accountModels "github.com/Roy6lty/minibank/modules/account"
	authModels "github.com/Roy6lty/minibank/modules/auth"
	transactionModels "github.com/Roy6lty/minibank/modules/transactions"
	transferModels "github.com/Roy6lty/minibank/modules/transfers"
)

type testRequest struct {
	Name string `json:"name" validate:"required"`
}

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	body := `{"name":"` + strings.Repeat("a", int(httpx.MaxJSONBodySize)) + `"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/", strings.NewReader(body))

	if httpx.DecodeJSON(recorder, request, &testRequest{}) {
		t.Fatal("expected oversized body to be rejected")
	}
	if recorder.Code != 413 {
		t.Fatalf("expected status 413, got %d", recorder.Code)
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Ada","extra":true}`))

	if httpx.DecodeJSON(recorder, request, &testRequest{}) {
		t.Fatal("expected unknown field to be rejected")
	}
	if recorder.Code != 400 {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestDecodeJSONRejectsSchemaViolations(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"name":""}`))

	if httpx.DecodeJSON(recorder, request, &testRequest{}) {
		t.Fatal("expected schema violation to be rejected")
	}
	if recorder.Code != 400 {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestDecodeJSONAcceptsValidBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Ada"}`))
	var destination testRequest

	if !httpx.DecodeJSON(recorder, request, &destination) {
		t.Fatalf("expected valid body to be accepted, got status %d", recorder.Code)
	}
	if destination.Name != "Ada" {
		t.Fatalf("expected decoded name Ada, got %q", destination.Name)
	}
}

func TestDecodeJSONValidatesRequestSchemas(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		destination any
	}{
		{
			name:        "registration requires a valid email",
			body:        `{"first_name":"Ada","last_name":"Lovelace","email":"invalid","password":"password"}`,
			destination: &authModels.RegisterRequest{},
		},
		{
			name:        "account creation restricts account type",
			body:        `{"account_type":"BROKERAGE","currency":"NGN"}`,
			destination: &accountModels.CreateAccountRequest{},
		},
		{
			name:        "transfer amount must be positive",
			body:        `{"idempotency_key":"request-1","sender_account_number":"1234567890","reciever_account_number":"0987654321","amount":0}`,
			destination: &transferModels.CreateTransferRequest{},
		},
		{
			name:        "transaction type is an enum",
			body:        `{"reference":"reference-1","type":"UNKNOWN","amount":1,"currency":"NGN"}`,
			destination: &transactionModels.CreateTransactionRequest{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/", strings.NewReader(test.body))

			if httpx.DecodeJSON(recorder, request, test.destination) {
				t.Fatal("expected schema violation to be rejected")
			}
			if recorder.Code != 400 {
				t.Fatalf("expected status 400, got %d", recorder.Code)
			}
		})
	}
}
