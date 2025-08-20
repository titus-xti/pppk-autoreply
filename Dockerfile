# Multi-stage build
FROM golang:1.24-alpine AS builder
WORKDIR /src

# Install build deps (git for go modules and postgres client)
RUN apk add --no-cache git ca-certificates postgresql-client && update-ca-certificates

# Cache mod downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binary (pure Go; CGO disabled)
ENV CGO_ENABLED=0
RUN go build -o /out/app .

# Runtime image with CA certs and postgres client
FROM alpine:3.19

# Install runtime deps (postgres client)
RUN apk add --no-cache postgresql-client ca-certificates

# Create app user
RUN addgroup -S appgroup && \
    adduser -S appuser -G appgroup

WORKDIR /app

# Copy binary
COPY --from=builder /out/app /app/app

# Switch to non-root user
USER appuser

# Run the application
ENTRYPOINT ["/app/app"]
