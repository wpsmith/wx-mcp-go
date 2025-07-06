#!/bin/bash

# Docker build script for swagger-docs-mcp
set -e

# Get version from git
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

echo "Building swagger-docs-mcp Docker image..."
echo "Version: $VERSION"
echo "Build time: $BUILD_TIME"

# Build the Docker image
docker build \
  --build-arg VERSION="$VERSION" \
  --build-arg BUILD_TIME="$BUILD_TIME" \
  --tag swagger-docs-mcp:latest \
  --tag swagger-docs-mcp:$VERSION \
  .

echo "Docker image built successfully!"
echo "Available tags:"
echo "  - swagger-docs-mcp:latest"
echo "  - swagger-docs-mcp:$VERSION"

# Show image size
docker images swagger-docs-mcp:latest --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"