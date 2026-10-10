# Multi-stage Dockerfile for AI Models Pulse
# 1. Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git and ca-certificates (needed for external HTTPS calls)
RUN apk add --no-cache git ca-certificates

# Copy dependency files
COPY go.mod ./
# (go.sum if present)

# Copy source code and embedded static assets
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o server main.go

# 2. Production runtime stage
FROM alpine:3.20

WORKDIR /app

# Install ca-certificates for outgoing HTTPS requests to Hugging Face, arXiv, etc.
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/server .

# Default port (can be overridden by cloud platform via PORT env var)
ENV PORT=8080

EXPOSE 8080

# Run server
ENTRYPOINT ["./server"]
