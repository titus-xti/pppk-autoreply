# Multi-stage build
FROM golang:1.23-alpine AS builder
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
FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /out/app /usr/local/bin/app

# Set a sane default working directory where the app will write session.db and qr_*.png
# Bind-mount a host ./data directory to /app to persist these files.
ENTRYPOINT ["/usr/local/bin/app"]
