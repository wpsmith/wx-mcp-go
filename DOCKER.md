# Docker Configuration for Swagger MCP Server

This document explains how to use the Swagger MCP Server with Docker for Claude Desktop integration.

## Quick Start

### 1. Build the Docker Image

```bash
# Build the image
./docker-build.sh

# Or manually:
docker build -t swagger-docs-mcp:latest .
```

### 2. Run with Docker

```bash
# Basic run
docker run -it --rm swagger-docs-mcp:latest

# With environment variables
docker run -it --rm \
  -e WX_MCP_PACKAGE_ID="db77e7e5-8c7f-42b1-b7e7-e58c7f32b140,ccab6ba7-fb31-4d56-ab6b-a7fb315d5685" \
  -e WX_MCP_API_KEY="your-api-key-here" \
  -e WX_MCP_DEBUG="false" \
  swagger-docs-mcp:latest
```

### 3. Run with Docker Compose

```bash
# Copy environment file
cp .env.example .env
# Edit .env with your settings

# Start the service
docker-compose up
```

## Claude Desktop Integration

### Option 1: Direct Docker Command

Add this to your Claude Desktop configuration file (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "swagger-docs": {
      "command": "docker",
      "args": [
        "run",
        "--rm",
        "-i",
        "--name", "swagger-mcp-server",
        "--env", "WX_MCP_PACKAGE_ID=your-package-ids",
        "--env", "WX_MCP_API_KEY=your-api-key",
        "--env", "WX_MCP_DEBUG=false",
        "swagger-docs-mcp:latest"
      ]
    }
  }
}
```

### Option 2: Docker Compose

```json
{
  "mcpServers": {
    "swagger-docs": {
      "command": "docker-compose",
      "args": [
        "-f", "/path/to/sun-mcp/docker-compose.yml",
        "run",
        "--rm",
        "swagger-mcp-server"
      ],
      "cwd": "/path/to/sun-mcp",
      "env": {
        "WX_MCP_PACKAGE_ID": "your-package-ids",
        "WX_MCP_API_KEY": "your-api-key"
      }
    }
  }
}
```

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `WX_MCP_PACKAGE_ID` | Comma-separated solara package IDs | `id1,id2,id3` |
| `WX_MCP_API_KEY` | Weather API key | `your-api-key` |
| `WX_MCP_DEBUG` | Enable debug logging | `true` or `false` |
| `WX_MCP_LOG_LEVEL` | Log level | `error`, `warn`, `info`, `debug` |
| `WX_MCP_PATHS` | Custom swagger paths | `/app/swagger_docs/v3` |
| `WX_MCP_TIMEOUT` | Request timeout (ms) | `30000` |
| `WX_MCP_MAX_TOOLS` | Maximum tools to generate | `1000` |

## Configuration Files

### .env File

Create a `.env` file in the project root:

```bash
# Solara package filtering
WX_MCP_PACKAGE_ID=db77e7e5-8c7f-42b1-b7e7-e58c7f32b140,ccab6ba7-fb31-4d56-ab6b-a7fb315d5685

# API Authentication
WX_MCP_API_KEY=your-api-key-here

# Logging
WX_MCP_DEBUG=false
WX_MCP_LOG_LEVEL=info

# Optional: Custom paths
# WX_MCP_PATHS=/app/swagger_docs/v3
```

### docker-compose.yml

The included `docker-compose.yml` provides:
- Environment variable configuration
- Volume mounting for swagger docs
- Network isolation
- Restart policies
- Health checks

## Volume Mounting

To use custom swagger documents:

```bash
docker run -it --rm \
  -v /path/to/your/swagger_docs:/app/swagger_docs:ro \
  -e WX_MCP_PATHS=/app/swagger_docs \
  swagger-docs-mcp:latest
```

## Health Checks

The Docker image includes health checks that verify the server can start:

```bash
# Check container health
docker ps --format "table {{.Names}}\t{{.Status}}"

# View health check logs
docker inspect --format='{{json .State.Health}}' swagger-mcp-server
```

## Debugging

### View Container Logs

```bash
# Real-time logs
docker logs -f swagger-mcp-server

# With docker-compose
docker-compose logs -f swagger-mcp-server
```

### Debug Mode

Enable debug logging:

```bash
docker run -it --rm \
  -e WX_MCP_DEBUG=true \
  -e WX_MCP_LOG_LEVEL=debug \
  swagger-docs-mcp:latest
```

### Interactive Shell

Access the container for debugging:

```bash
# Start container with shell
docker run -it --rm --entrypoint /bin/sh swagger-docs-mcp:latest

# Or exec into running container
docker exec -it swagger-mcp-server /bin/sh
```

## Performance Considerations

### Resource Limits

Set memory and CPU limits:

```yaml
# In docker-compose.yml
services:
  swagger-mcp-server:
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: '0.5'
        reservations:
          memory: 256M
          cpus: '0.25'
```

### Optimization

- Use specific package IDs to reduce memory usage
- Limit swagger paths to only needed versions
- Set appropriate tool limits

## Security

### Non-Root User

The container runs as a non-root user (`mcp:nodejs`) for security.

### Read-Only Volumes

Mount swagger docs as read-only:

```bash
-v /path/to/swagger_docs:/app/swagger_docs:ro
```

### Environment Variables

Store sensitive data in environment files or secrets:

```bash
# Use environment file
docker run --env-file .env swagger-docs-mcp:latest

# Or with docker-compose
docker-compose --env-file .env up
```

## Troubleshooting

### Common Issues

1. **Container exits immediately**
   - Check logs: `docker logs swagger-mcp-server`
   - Verify environment variables
   - Ensure swagger docs are accessible

2. **Claude Desktop can't connect**
   - Verify Docker is running
   - Check container status: `docker ps`
   - Ensure container name matches config

3. **Permission errors**
   - Check file ownership in volumes
   - Verify user permissions (container runs as UID 1001)

4. **Memory issues**
   - Increase Docker memory limits
   - Use package ID filtering to reduce load
   - Limit swagger paths

### Useful Commands

```bash
# View container resource usage
docker stats swagger-mcp-server

# Inspect container configuration
docker inspect swagger-mcp-server

# View image layers
docker history swagger-docs-mcp:latest

# Clean up unused images
docker image prune
```

## Updates

To update the MCP server:

```bash
# Rebuild image
./docker-build.sh

# Or pull latest (if published)
docker pull swagger-docs-mcp:latest

# Restart services
docker-compose down && docker-compose up
```