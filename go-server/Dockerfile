FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o checkin-kiosk-api ./cmd

FROM alpine:3.19
WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /app/checkin-kiosk-api .
# properties.yaml is mounted in at runtime — the image itself does not
# carry a property registry. See docker-compose.yml or k8s ConfigMap.
RUN adduser -D -g '' appuser
USER appuser
EXPOSE 8089
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8089/health || exit 1
CMD ["./checkin-kiosk-api"]
