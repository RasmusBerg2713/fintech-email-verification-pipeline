package verification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strings"
	"sync"
	"time"
)

const ReviewThreshold = 70

type PaymentEvent struct {
	EventID     string    `json:"event_id"`
	Kind        string    `json:"kind"`
	AmountMinor int64     `json:"amount_minor"`
	Currency    string    `json:"currency"`
	RiskScore   int       `json:"risk_score"`
	OccurredAt  time.Time `json:"occurred_at"`
}

type SignupInput struct {
	AccountID string       `json:"account_id"`
	Email     string       `json:"email"`
	Payment   PaymentEvent `json:"payment_event"`
}

type SignupResult struct {
	AccountID  string     `json:"account_id"`
	State      string     `json:"state"`
	MessageID  string     `json:"message_id,omitempty"`
	AuditEvent AuditEvent `json:"audit_event"`
}

type AuditEvent struct {
	AccountID      string    `json:"account_id"`
	PaymentEventID string    `json:"payment_event_id"`
	Decision       string    `json:"decision"`
	NotificationID string    `json:"notification_id,omitempty"`
	RecordedAt     time.Time `json:"recorded_at"`
}

type Mailer interface {
	EmailSend(context.Context, SendEmailInput, string) (SendEmailResult, error)
	EmailEventList(context.Context, string) (json.RawMessage, error)
}

type tokenSource func() (string, error)

type Flow struct {
	mailer        Mailer
	publicBaseURL string
	now           func() time.Time
	newToken      tokenSource
	mu            sync.RWMutex
	messages      map[string]string
}

func NewFlow(mailer Mailer, publicBaseURL string) *Flow {
	return &Flow{
		mailer:        mailer,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		now:           time.Now,
		newToken:      randomToken,
		messages:      make(map[string]string),
	}
}

func (f *Flow) StartSignup(ctx context.Context, input SignupInput) (SignupResult, error) {
	decision, err := decide(input)
	if err != nil {
		return SignupResult{}, err
	}
	audit := AuditEvent{
		AccountID: input.AccountID, PaymentEventID: input.Payment.EventID,
		Decision: decision, RecordedAt: f.now().UTC(),
	}
	if decision == "manual_review" {
		return SignupResult{AccountID: input.AccountID, State: decision, AuditEvent: audit}, nil
	}

	token, err := f.newToken()
	if err != nil {
		return SignupResult{}, fmt.Errorf("create verification token: %w", err)
	}
	verifyURL := f.publicBaseURL + "/verify?token=" + url.QueryEscape(token)
	mail := SendEmailInput{
		To:      input.Email,
		Subject: "Verify your email for the payment account",
		HTML:    "<p>Confirm this email before enabling sensitive payment actions.</p><p><a href=\"" + html.EscapeString(verifyURL) + "\">Verify email</a></p>",
	}
	sent, err := f.mailer.EmailSend(ctx, mail, "signup-payment-"+input.Payment.EventID)
	if err != nil {
		return SignupResult{}, err
	}
	audit.NotificationID = sent.MessageID
	f.mu.Lock()
	f.messages[input.AccountID] = sent.MessageID
	f.mu.Unlock()
	return SignupResult{
		AccountID: input.AccountID, State: "verification_pending",
		MessageID: sent.MessageID, AuditEvent: audit,
	}, nil
}

func (f *Flow) DeliveryEvents(ctx context.Context, accountID string) (json.RawMessage, error) {
	f.mu.RLock()
	messageID := f.messages[accountID]
	f.mu.RUnlock()
	if messageID == "" {
		return nil, fmt.Errorf("no verification notification for account")
	}
	return f.mailer.EmailEventList(ctx, messageID)
}

func decide(input SignupInput) (string, error) {
	if strings.TrimSpace(input.AccountID) == "" || strings.TrimSpace(input.Email) == "" {
		return "", fmt.Errorf("account_id and email are required")
	}
	if input.Payment.EventID == "" || input.Payment.Kind != "payment_authorized" {
		return "", fmt.Errorf("an authorized payment event is required")
	}
	if input.Payment.AmountMinor <= 0 || input.Payment.Currency == "" || input.Payment.OccurredAt.IsZero() {
		return "", fmt.Errorf("payment event is incomplete")
	}
	if input.Payment.RiskScore < 0 || input.Payment.RiskScore > 100 {
		return "", fmt.Errorf("risk_score must be between 0 and 100")
	}
	if input.Payment.RiskScore >= ReviewThreshold {
		return "manual_review", nil
	}
	return "send_verification", nil
}

func randomToken() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
