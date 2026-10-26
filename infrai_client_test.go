package verification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientRequestBoundary(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/email/send" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Idempotency-Key") != "signup-payment-pay_9" {
			t.Fatal("required request headers missing")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 3 || body["to"] != "alex@example.com" || body["subject"] != "Verify" || body["html"] != "<p>Link</p>" {
			t.Fatalf("body = %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"data":null,"error":{"code":"rate_limited","message":"retry later"},"metadata":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"message_id":"msg_42"},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL
	client.MaxRetries = 1
	client.Sleep = func(context.Context, time.Duration) error { return nil }
	result, err := client.EmailSend(context.Background(), SendEmailInput{
		To: "alex@example.com", Subject: "Verify", HTML: "<p>Link</p>",
	}, "signup-payment-pay_9")
	if err != nil {
		t.Fatal(err)
	}
	if result.MessageID != "msg_42" || attempts != 2 {
		t.Fatalf("result=%#v attempts=%d", result, attempts)
	}
}

func TestClientDecodesBusinessErrorBeforeStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"ok":false,"data":null,"error":{"code":"policy_rejected","message":"review required"},"metadata":{}}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL
	_, err := client.EmailEventList(context.Background(), "msg_42")
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.HTTPStatus != http.StatusUnprocessableEntity || apiErr.Code != "policy_rejected" {
		t.Fatalf("error = %#v", err)
	}
}
