# Build stage
FROM golang:1.23-alpine AS builder

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

# Build the application with build information
ARG MAJOR=1
ARG MINOR=0
ARG PATCH=0
ARG PRERELEASE=
ARG COMMIT_COUNT=0
ARG BUILD_TIME
ARG COMMIT_HASH
ARG BUILD_USER=docker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w \
    -X swagger-docs-mcp/pkg/version.Major=${MAJOR} \
    -X swagger-docs-mcp/pkg/version.Minor=${MINOR} \
    -X swagger-docs-mcp/pkg/version.Patch=${PATCH} \
    -X swagger-docs-mcp/pkg/version.PreRelease=${PRERELEASE} \
    -X swagger-docs-mcp/pkg/version.CommitCount=${COMMIT_COUNT} \
    -X swagger-docs-mcp/pkg/version.BuildDate=${BUILD_TIME} \
    -X swagger-docs-mcp/pkg/version.CommitHash=${COMMIT_HASH} \
    -X swagger-docs-mcp/pkg/version.BuildUser=${BUILD_USER}" \
    -o swagger-docs-mcp .

# Final stage
FROM alpine:latest

# Install ca-certificates and wget for health checks
RUN apk --no-cache add ca-certificates wget

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

# Expose port for SSE mode
EXPOSE 8080

# Health check for SSE mode - try health endpoint, fallback to help for MCP mode
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || \
      ./swagger-docs-mcp --help > /dev/null || exit 1

# Set entrypoint and default command for SSE mode
ENTRYPOINT ["./swagger-docs-mcp"]
CMD ["--sse", "--port=8080"]