#!/bin/sh
set -eu

service_url="${SERVICE_URL:-http://localhost:8080}"
occurred_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/json' \
  --data "{\"account_id\":\"acct_demo_17\",\"email\":\"${DEMO_EMAIL_TO:?DEMO_EMAIL_TO is required}\",\"payment_event\":{\"event_id\":\"pay_demo_9\",\"kind\":\"payment_authorized\",\"amount_minor\":2500,\"currency\":\"USD\",\"risk_score\":18,\"occurred_at\":\"${occurred_at}\"}}" \
  "${service_url}/signup"

printf '\n'
curl --fail-with-body \
  --request GET \
  "${service_url}/accounts/acct_demo_17/email-events"
printf '\n'
