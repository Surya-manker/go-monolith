# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Cache dependency downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o invobill .

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

COPY --from=builder /app/invobill    ./invobill
COPY --from=builder /app/templates   ./templates
COPY --from=builder /app/static      ./static

# Writable data directory for uploads / generated files.
RUN mkdir -p data

EXPOSE 8080

ENV PORT=8080 \
    TZ=UTC

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["./invobill"]
