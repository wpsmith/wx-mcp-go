# Build stage
FROM golang:1.21-alpine AS builder

# Install ca-certificates for SSL/TLS connections
RUN apk add --no-cache ca-certificates git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o swagger-docs-mcp .

# Final stage
FROM alpine:latest

# Install ca-certificates for SSL/TLS connections
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 -S mcp && \
    adduser -u 1001 -S mcp -G mcp

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/swagger-docs-mcp .

# Change ownership to non-root user
RUN chown -R mcp:mcp /app

# Switch to non-root user
USER mcp

# Expose port (not needed for MCP but good practice)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD ./swagger-docs-mcp --help > /dev/null || exit 1

# Set entrypoint and default command
ENTRYPOINT ["./swagger-docs-mcp"]
CMD []