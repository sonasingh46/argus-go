# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install ca-certificates for HTTPS connections
RUN apk add --no-cache ca-certificates

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
# CGO_ENABLED=0 for static binary compatible with distroless
# -ldflags for smaller binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/argus \
    ./cmd/argus

# Runtime stage - distroless nonroot image
FROM gcr.io/distroless/static:nonroot

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/argus /app/argus

# Copy default config
COPY --from=builder /app/config/config.yaml /app/config/config.yaml

# Expose the default port
EXPOSE 8080

# Run as nonroot user (distroless nonroot runs as uid 65532)
USER nonroot:nonroot

# Set the entrypoint
ENTRYPOINT ["/app/argus"]

# Default command arguments
CMD ["-config", "/app/config/config.yaml"]
