# Multi-stage build
FROM golang:1.24-alpine AS builder
WORKDIR /src

# Install build deps (git for go modules)
RUN apk add --no-cache git ca-certificates && update-ca-certificates

# Cache mod downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binary (pure Go; CGO disabled)
ENV CGO_ENABLED=0
RUN go build -o /out/app .

# Runtime image with CA certs
FROM alpine:3.19

# Create app user and directory with proper permissions
RUN addgroup -S appgroup && \
    adduser -S appuser -G appgroup && \
    mkdir -p /data && \
    chown -R appuser:appgroup /data

WORKDIR /data

# Copy binary
COPY --from=builder /out/app /app

# Switch to non-root user
USER appuser

# Run the application
ENTRYPOINT ["/app"]
