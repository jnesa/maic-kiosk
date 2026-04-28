#!/usr/bin/env bash
# End-to-end smoke for the kiosk API. Exits non-zero on the first failure.
# Requires: jq, curl. Set BASE / DEVICE_KEY / RESERVATION_ID / LAST_NAME beforehand.
set -euo pipefail

BASE=${BASE:-http://localhost:8089}
DEVICE_KEY=${DEVICE_KEY:?set DEVICE_KEY to the device shared secret}
LAST_NAME=${LAST_NAME:?set LAST_NAME to a guest expected to arrive today}

hdr=(-H "X-Device-Key: ${DEVICE_KEY}" -H "Content-Type: application/json")

echo "→ /health"
curl -fsS "${BASE}/health" | jq .

echo "→ /api/v1/ready"
curl -fsS "${BASE}/api/v1/ready" | jq .

echo "→ /api/v1/sessions/lookup (last name)"
LOOKUP=$(curl -fsS "${hdr[@]}" -X POST "${BASE}/api/v1/sessions/lookup" -d "{\"lastName\":\"${LAST_NAME}\"}")
echo "${LOOKUP}" | jq .

RESULT=$(echo "${LOOKUP}" | jq -r .result)
if [[ "${RESULT}" == "matched" ]]; then
  TOKEN=$(echo "${LOOKUP}" | jq -r .token)
elif [[ "${RESULT}" == "ambiguous" ]]; then
  CAND_TOKEN=$(echo "${LOOKUP}" | jq -r .candidateToken)
  CAND_ID=$(echo "${LOOKUP}" | jq -r '.candidates[0].candidateId')
  echo "→ /api/v1/sessions/select"
  PICK=$(curl -fsS "${hdr[@]}" -X POST "${BASE}/api/v1/sessions/select" -d "{\"candidateToken\":\"${CAND_TOKEN}\",\"candidateId\":\"${CAND_ID}\"}")
  echo "${PICK}" | jq .
  TOKEN=$(echo "${PICK}" | jq -r .token)
else
  echo "no reservation found — kiosk would route to /lookup/not-found"
  exit 0
fi

AUTH=("Authorization: Bearer ${TOKEN}")

echo "→ /api/v1/sessions/me/form"
curl -fsS -H "X-Device-Key: ${DEVICE_KEY}" -H "${AUTH[@]}" "${BASE}/api/v1/sessions/me/form" | jq '.config | keys' >/dev/null
echo "  ok"

echo "→ /api/v1/sessions/me/guest (primary)"
curl -fsS -H "X-Device-Key: ${DEVICE_KEY}" -H "${AUTH[@]}" -H "Content-Type: application/json" \
  -X POST "${BASE}/api/v1/sessions/me/guest" \
  -d '{"guestIndex":0,"guest":{"id":null,"fname":"Mario","lname":"'"${LAST_NAME}"'","dob":"1985-03-12","country":"IT","nationality":"ITA","city":"Roma","postal":"00100","street":"Via Roma","house_number":"1","document":1,"document_id":"AB1234567","document_issuer":"Comune di Roma","document_issue_date":"2020-01-15","phone":"","title":"","traveltime_changed":false,"traveltime_arrival":"","traveltime_departure":"","specialtravel":false,"special_travel_event_id":"","businesstravel":false,"annualcard":false,"annualcard_number":"","handicap":false,"handicap_needhelp":false,"handicap_number":"","handicap_is_help":false}}' \
  | jq .

echo "→ /api/v1/sessions/me/firm"
curl -fsS -H "X-Device-Key: ${DEVICE_KEY}" -H "${AUTH[@]}" -H "Content-Type: application/json" \
  -X POST "${BASE}/api/v1/sessions/me/firm" \
  -d '{"firm":{"compname":"","vatid":"","address":"","city":"","arrival":"","arrival_via":"","arrival_with_car":false,"phone":"","email":"","useFirmForBilling":false,"useAnotherBillingAddress":false,"billing_address":"","transfer":false,"transferText":"","babyBed":false,"babyBedText":"","dogPackage":false,"dogPackageText":"","alergies":false,"alergiesText":"","accessible":false,"additionalLinens":false,"additionalLinensAmount":"","preferedCommunication":"","signature":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="}}' \
  | jq .

echo "→ /api/v1/sessions/me/submit"
curl -fsS -H "X-Device-Key: ${DEVICE_KEY}" -H "${AUTH[@]}" -H "X-Lookup-Method: last_name" -H "X-Kiosk-Language: en" \
  -X POST "${BASE}/api/v1/sessions/me/submit" | jq .

echo "✓ end-to-end OK"
