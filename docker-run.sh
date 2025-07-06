#!/bin/bash

# Docker run script for swagger-docs-mcp
set -e

# Default values
IMAGE_NAME="swagger-docs-mcp:latest"
CONTAINER_NAME="swagger-mcp-server"
ENV_FILE=".env"

# Function to show usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo "Options:"
    echo "  -e, --env-file FILE    Environment file (default: .env)"
    echo "  -n, --name NAME        Container name (default: swagger-mcp-server)"
    echo "  -i, --image IMAGE      Docker image (default: swagger-docs-mcp:latest)"
    echo "  -d, --detach           Run in detached mode"
    echo "  -h, --help             Show this help"
    echo ""
    echo "Examples:"
    echo "  $0                     # Run interactively with .env file"
    echo "  $0 -d                  # Run in detached mode"
    echo "  $0 -e production.env   # Use specific environment file"
}

# Parse command line arguments
DETACHED=false
while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--env-file)
            ENV_FILE="$2"
            shift 2
            ;;
        -n|--name)
            CONTAINER_NAME="$2"
            shift 2
            ;;
        -i|--image)
            IMAGE_NAME="$2"
            shift 2
            ;;
        -d|--detach)
            DETACHED=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Check if image exists
if ! docker images "$IMAGE_NAME" --format "{{.Repository}}:{{.Tag}}" | grep -q "$IMAGE_NAME"; then
    echo "Error: Docker image '$IMAGE_NAME' not found"
    echo "Build the image first with: ./docker-build.sh"
    exit 1
fi

# Stop and remove existing container if it exists
if docker ps -a --format "{{.Names}}" | grep -q "^${CONTAINER_NAME}$"; then
    echo "Stopping existing container: $CONTAINER_NAME"
    docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
    echo "Removing existing container: $CONTAINER_NAME"
    docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
fi

# Prepare docker run command
DOCKER_CMD="docker run"

# Add environment file if it exists
if [[ -f "$ENV_FILE" ]]; then
    echo "Using environment file: $ENV_FILE"
    DOCKER_CMD="$DOCKER_CMD --env-file $ENV_FILE"
else
    echo "Warning: Environment file '$ENV_FILE' not found"
fi

# Add common options
DOCKER_CMD="$DOCKER_CMD --name $CONTAINER_NAME"
DOCKER_CMD="$DOCKER_CMD --rm"

# Add interactive/detached mode
if [[ "$DETACHED" == "true" ]]; then
    DOCKER_CMD="$DOCKER_CMD -d"
    echo "Starting container in detached mode..."
else
    DOCKER_CMD="$DOCKER_CMD -it"
    echo "Starting container in interactive mode..."
fi

# Add image name
DOCKER_CMD="$DOCKER_CMD $IMAGE_NAME"

# Run the container
echo "Running: $DOCKER_CMD"
eval $DOCKER_CMD

# Show container status if detached
if [[ "$DETACHED" == "true" ]]; then
    echo ""
    echo "Container started successfully!"
    echo "Container name: $CONTAINER_NAME"
    echo ""
    echo "Useful commands:"
    echo "  docker logs $CONTAINER_NAME           # View logs"
    echo "  docker logs -f $CONTAINER_NAME        # Follow logs"
    echo "  docker exec -it $CONTAINER_NAME sh    # Access shell"
    echo "  docker stop $CONTAINER_NAME           # Stop container"
    echo ""
    echo "Current status:"
    docker ps --filter "name=$CONTAINER_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
fi