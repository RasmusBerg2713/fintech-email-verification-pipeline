package verification

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type fakeMailer struct {
	sends     int
	lastInput SendEmailInput
	lastKey   string
	eventID   string
}

func (f *fakeMailer) EmailSend(_ context.Context, input SendEmailInput, key string) (SendEmailResult, error) {
	f.sends++
	f.lastInput = input
	f.lastKey = key
	return SendEmailResult{MessageID: "msg_42"}, nil
}

func (f *fakeMailer) EmailEventList(_ context.Context, messageID string) (json.RawMessage, error) {
	f.eventID = messageID
	return json.RawMessage(`[{"type":"delivered"}]`), nil
}

func TestSignupRiskDecision(t *testing.T) {
	now := time.Date(2026, 9, 4, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		riskScore int
		wantState string
		wantSends int
	}{
		{name: "low risk sends verification", riskScore: 18, wantState: "verification_pending", wantSends: 1},
		{name: "threshold enters review", riskScore: 70, wantState: "manual_review", wantSends: 0},
		{name: "higher risk enters review", riskScore: 94, wantState: "manual_review", wantSends: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mailer := &fakeMailer{}
			flow := NewFlow(mailer, "http://localhost:8080")
			flow.now = func() time.Time { return now }
			flow.newToken = func() (string, error) { return "fixed-token", nil }
			input := SignupInput{
				AccountID: "acct_17", Email: "alex@example.com",
				Payment: PaymentEvent{
					EventID: "pay_9", Kind: "payment_authorized", AmountMinor: 2500,
					Currency: "USD", RiskScore: tt.riskScore, OccurredAt: now,
				},
			}
			got, err := flow.StartSignup(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != tt.wantState || mailer.sends != tt.wantSends {
				t.Fatalf("state=%q sends=%d, want state=%q sends=%d", got.State, mailer.sends, tt.wantState, tt.wantSends)
			}
			if mailer.sends == 1 {
				if mailer.lastKey != "signup-payment-pay_9" {
					t.Fatalf("idempotency key = %q", mailer.lastKey)
				}
				if got.AuditEvent.NotificationID != "msg_42" {
					t.Fatalf("notification id = %q", got.AuditEvent.NotificationID)
				}
				if _, err := flow.DeliveryEvents(context.Background(), input.AccountID); err != nil {
					t.Fatal(err)
				}
				if mailer.eventID != "msg_42" {
					t.Fatalf("event lookup used %q", mailer.eventID)
				}
			}
		})
	}
}
