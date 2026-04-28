# checkin-kiosk-api — multi-tenant kiosk proxy

Small Go service. Reads a YAML registry of hotel properties and proxies
the kiosk SPA's calls to each property's legacy PMSApi `/api/kiosk/*`
endpoints, adding the per-property device key.

The browser only ever talks to this service. Per-property device keys
stay server-side.

## Layout

```
.
├── cmd/main.go                — bootstrap + graceful shutdown
├── internal/
│   ├── config/                — loads properties.yaml + env, validates
│   ├── proxy/                 — http.Client wrapper + allowlisted forwarder
│   └── server/                — chi router, slug middleware, handlers
├── properties.example.yaml    — copy → properties.yaml, fill secrets
├── go.mod
├── Dockerfile                 — golang:1.25-alpine → alpine:3.19
└── README.md
```

## Run

```bash
SMART_MOOV_DEVICE_KEY="$(openssl rand -hex 32)" \
go run ./cmd

# or
cp properties.example.yaml properties.yaml
docker compose up
```

The service exits if `properties.yaml` is missing, malformed, or any
referenced env var fails to resolve. That's intentional — a kiosk that
boots half-broken is worse than one that doesn't boot.

## Env

| Var | Required | Default | Purpose |
| --- | --- | --- | --- |
| `PORT` | no | `8089` | listen port |
| `ENV` | no | `dev` | logged for observability |
| `PROPERTIES_FILE` | no | `properties.yaml` | path to registry |
| `ALLOWED_ORIGINS` | no | `*` | CORS allowlist (comma-separated) |
| `UPSTREAM_TIMEOUT_MS` | no | `12000` | per-request timeout to legacy PMSApi |
| `<PROP>_DEVICE_KEY`     | yes | — | one per property referenced from YAML |

## Endpoints

See [../README.md](../README.md) for the full
matrix. In short: `/health`, `/api/kiosk/v1/{ready,properties}`, then
slug-scoped `/api/kiosk/v1/{slug}/{config,lookup,select,form,save-guest,save-firm,submit}`.

## Smoke

```bash
curl http://localhost:8089/health
curl http://localhost:8089/api/kiosk/v1/properties
curl http://localhost:8089/api/kiosk/v1/smart-moov/config
curl -X POST http://localhost:8089/api/kiosk/v1/smart-moov/lookup \
  -H 'Content-Type: application/json' \
  -d '{"lastName":"Mustermann"}'
```

The last call is forwarded to
`{pmsapi_url}/api/kiosk/lookup` with `X-Device-Key: $SMART_MOOV_DEVICE_KEY`.
