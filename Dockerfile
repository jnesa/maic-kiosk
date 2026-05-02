# MAIC Kiosk — single image: stateless Go server + both SPA bundles.
#
# Build stages:
#   1. node-builder — compiles kiosk-spa/dist and admin-spa/dist
#   2. go-builder   — compiles cmd/server (the only binary)
#   3. runtime      — Alpine final image, stateless (no /data volume)
#
# The runtime image runs cmd/server, which forwards to the legacy MAIC
# PHP monolith at $LEGACY_BASE_URL (default https://dev.maiccube.com)
# and serves both SPA bundles from baked-in directories.

# ─── 1. Build SPA bundles ─────────────────────────────────────────────
FROM node:20-alpine AS node-builder
WORKDIR /build

# Copy + install kiosk SPA deps first so Docker layer caching keeps the
# install step out of the way of source edits.
COPY kiosk-spa/package*.json ./kiosk-spa/
RUN cd kiosk-spa && npm ci

COPY admin-spa/package*.json ./admin-spa/
RUN cd admin-spa && npm ci

COPY kiosk-spa ./kiosk-spa
COPY admin-spa ./admin-spa
RUN cd kiosk-spa && npm run build
RUN cd admin-spa && npm run build

# ─── 2. Build Go binary ──────────────────────────────────────────────
FROM golang:1.25-alpine AS go-builder
WORKDIR /src
RUN apk add --no-cache git
COPY go-server/go.mod go-server/go.sum ./
RUN go mod download
COPY go-server ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /out/server ./cmd/server

# ─── 3. Runtime image ────────────────────────────────────────────────
FROM alpine:3.19
WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata wget && \
    adduser -D -g '' appuser

COPY --from=go-builder  /out/server          /app/server
COPY --from=node-builder /build/kiosk-spa/dist /app/kiosk-spa-dist
COPY --from=node-builder /build/admin-spa/dist /app/admin-spa-dist

ENV LEGACY_BASE_URL=https://dev.maiccube.com \
    KIOSK_SPA_DIR=/app/kiosk-spa-dist \
    ADMIN_SPA_DIR=/app/admin-spa-dist \
    PORT=8089

EXPOSE 8089
USER appuser
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8089/api/kiosk/v1/health || exit 1

CMD ["/app/server"]
