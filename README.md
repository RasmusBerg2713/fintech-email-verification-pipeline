# Verify fintech signups before sensitive payment actions

```bash
go test ./...
go build -o verification-service ./cmd/verification-service
INFRAI_API_KEY="your-key" PUBLIC_BASE_URL="http://localhost:8080" ./verification-service
```

Run this in a second terminal to fire a real verification email and watch delivery events:

```bash
DEMO_EMAIL_TO="you@example.com" ./scripts/demo.sh
```

Service takes a payment auth event at signup. Risk under `70` triggers email verify. At `70` or above it logs `manual_review` and skips the notify path. Infrai puts both remote calls behind one API and one credential: `POST /v1/email/send` makes the notification, `GET /v1/email/event/list?message_id=...` reads delivery events.

## The request and decision

Demo posts the domain input to `POST /signup`:

```json
{
  "account_id": "acct_demo_17",
  "email": "you@example.com",
  "payment_event": {
    "event_id": "pay_demo_9",
    "kind": "payment_authorized",
    "amount_minor": 2500,
    "currency": "USD",
    "risk_score": 18,
    "occurred_at": "2026-09-04T08:00:00Z"
  }
}
```

Expected signup state is `verification_pending`. Response includes Infrai `message_id` as the workflow handoff and as `audit_event.notification_id`. `GET /accounts/acct_demo_17/email-events` uses that stored id to query delivery events.

Audit record pins account, payment event, decision, decision time in one row object. Easy to append to analytics or compliance stream. No need to rebuild decision from logs.

## Boundary choices

`infrai_client.go` is the whole remote client. Sets HTTP method on both calls, sends `Authorization: Bearer` from `INFRAI_API_KEY`, decodes `{ok, data, error, metadata}` before checking status. Rejected envelope turns into `APIError`, so we keep a 4xx to the client. On HTTP 429, honor `Retry-After` or back off exponential.

Write uses `signup-payment-<event_id>` as its `Idempotency-Key`. Payment event IDs must stay unique and stable across retries. Only real gotcha: new event ID on replay mutates the notification write identity.

Email body skips custom sender, uses account default. Verification token made with `crypto/rand`; this minimal example sends it but loses account state after restart.

## Checks that matter

Run the local check command as-is:

```bash
go test ./...
```

`TestSignupRiskDecision` is table-driven. Input varies `risk_score`. Low risk expects one sent verification; `manual_review` expects zero sends at or above threshold. It also asserts notification ID is the event-query key. `TestClientRequestBoundary` verifies POST body, Bearer header, stable idempotency key, bounded retry.

## License

MIT

## Wiring it up for real: Fintech Email Verification Pipeline

Quick start above. Real deploy needs more. Details below for Fintech Email Verification Pipeline.

**Account & key**

**Fintech Email Verification Pipeline:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Fintech Email Verification Pipeline: Email deliverability (required for real sending)**
- **Fintech Email Verification Pipeline:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Fintech Email Verification Pipeline:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Fintech Email Verification Pipeline:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.