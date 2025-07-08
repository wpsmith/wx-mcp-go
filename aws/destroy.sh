#!/bin/bash

# AWS Destroy Script for Swagger Docs MCP
set -e

REGION="us-east-1"
STACK_NAME="swagger-docs-mcp"
ECR_REPO="swagger-docs-mcp"
LAMBDA_FUNCTION_NAME="swagger-docs-mcp-proxy"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    if ! command -v aws &> /dev/null; then
        error "AWS CLI not found. Please install it first."
        exit 1
    fi
    
    # Check AWS credentials
    if ! aws sts get-caller-identity &> /dev/null; then
        error "AWS credentials not configured or invalid"
        exit 1
    fi
    
    log "Prerequisites check passed"
}

# Delete CloudFormation stack
delete_stack() {
    log "Checking CloudFormation stack status..."
    
    # Check if stack exists
    if ! aws cloudformation describe-stacks --stack-name ${STACK_NAME} --region ${REGION} &> /dev/null; then
        warn "CloudFormation stack '${STACK_NAME}' does not exist"
        return 0
    fi
    
    # Get stack status
    STACK_STATUS=$(aws cloudformation describe-stacks \
        --stack-name ${STACK_NAME} \
        --query 'Stacks[0].StackStatus' \
        --output text \
        --region ${REGION})
    
    log "Current stack status: ${STACK_STATUS}"
    
    case "${STACK_STATUS}" in
        "ROLLBACK_COMPLETE"|"CREATE_FAILED"|"ROLLBACK_FAILED"|"DELETE_FAILED")
            log "Stack is in ${STACK_STATUS} state, deleting directly..."
            aws cloudformation delete-stack \
                --stack-name ${STACK_NAME} \
                --region ${REGION}
            ;;
        "DELETE_IN_PROGRESS")
            log "Stack deletion already in progress..."
            ;;
        "DELETE_COMPLETE")
            log "Stack already deleted"
            return 0
            ;;
        *)
            log "Deleting CloudFormation stack..."
            aws cloudformation delete-stack \
                --stack-name ${STACK_NAME} \
                --region ${REGION}
            ;;
    esac
    
    log "Waiting for stack deletion to complete..."
    aws cloudformation wait stack-delete-complete \
        --stack-name ${STACK_NAME} \
        --region ${REGION} \
        --cli-read-timeout 0 \
        --cli-connect-timeout 60
    
    log "CloudFormation stack deleted successfully"
}

# Delete Lambda function
delete_lambda() {
    log "Checking Lambda function..."
    
    if aws lambda get-function --function-name ${LAMBDA_FUNCTION_NAME} --region ${REGION} &> /dev/null; then
        log "Deleting Lambda function: ${LAMBDA_FUNCTION_NAME}"
        aws lambda delete-function \
            --function-name ${LAMBDA_FUNCTION_NAME} \
            --region ${REGION}
        log "Lambda function deleted"
    else
        warn "Lambda function '${LAMBDA_FUNCTION_NAME}' does not exist"
    fi
}

# Delete ECR repository
delete_ecr() {
    log "Checking ECR repository..."
    
    if aws ecr describe-repositories --repository-names ${ECR_REPO} --region ${REGION} &> /dev/null; then
        log "Deleting ECR repository: ${ECR_REPO}"
        
        # Delete all images first
        IMAGE_TAGS=$(aws ecr list-images \
            --repository-name ${ECR_REPO} \
            --query 'imageIds[].imageTag' \
            --output text \
            --region ${REGION} 2>/dev/null || echo "")
        
        if [ -n "$IMAGE_TAGS" ] && [ "$IMAGE_TAGS" != "None" ]; then
            log "Deleting images from ECR repository..."
            for tag in $IMAGE_TAGS; do
                aws ecr batch-delete-image \
                    --repository-name ${ECR_REPO} \
                    --image-ids imageTag=$tag \
                    --region ${REGION} &> /dev/null || true
            done
        fi
        
        # Delete untagged images
        UNTAGGED_IMAGES=$(aws ecr list-images \
            --repository-name ${ECR_REPO} \
            --filter tagStatus=UNTAGGED \
            --query 'imageIds[].imageDigest' \
            --output text \
            --region ${REGION} 2>/dev/null || echo "")
        
        if [ -n "$UNTAGGED_IMAGES" ] && [ "$UNTAGGED_IMAGES" != "None" ]; then
            log "Deleting untagged images from ECR repository..."
            for digest in $UNTAGGED_IMAGES; do
                aws ecr batch-delete-image \
                    --repository-name ${ECR_REPO} \
                    --image-ids imageDigest=$digest \
                    --region ${REGION} &> /dev/null || true
            done
        fi
        
        # Delete repository
        aws ecr delete-repository \
            --repository-name ${ECR_REPO} \
            --force \
            --region ${REGION}
        log "ECR repository deleted"
    else
        warn "ECR repository '${ECR_REPO}' does not exist"
    fi
}

# Clean up local Docker images
cleanup_docker() {
    log "Cleaning up local Docker images..."
    
    # Remove local images if they exist
    if docker images | grep -q ${ECR_REPO}; then
        log "Removing local Docker images for ${ECR_REPO}..."
        docker images --format "table {{.Repository}}:{{.Tag}}" | grep ${ECR_REPO} | while read image; do
            if [ -n "$image" ]; then
                docker rmi "$image" 2>/dev/null || true
            fi
        done
    fi
    
    # Clean up dangling images
    DANGLING_IMAGES=$(docker images -f "dangling=true" -q 2>/dev/null || echo "")
    if [ -n "$DANGLING_IMAGES" ]; then
        log "Removing dangling Docker images..."
        docker rmi $DANGLING_IMAGES 2>/dev/null || true
    fi
    
    log "Docker cleanup complete"
}

# Show current resources before deletion
show_resources() {
    log "Current AWS resources for project:"
    echo ""
    
    # CloudFormation stack
    if aws cloudformation describe-stacks --stack-name ${STACK_NAME} --region ${REGION} &> /dev/null; then
        STACK_STATUS=$(aws cloudformation describe-stacks \
            --stack-name ${STACK_NAME} \
            --query 'Stacks[0].StackStatus' \
            --output text \
            --region ${REGION})
        echo "  CloudFormation Stack: ${STACK_NAME} (${STACK_STATUS})"
    else
        echo "  CloudFormation Stack: Not found"
    fi
    
    # Lambda function
    if aws lambda get-function --function-name ${LAMBDA_FUNCTION_NAME} --region ${REGION} &> /dev/null; then
        echo "  Lambda Function: ${LAMBDA_FUNCTION_NAME}"
    else
        echo "  Lambda Function: Not found"
    fi
    
    # ECR repository
    if aws ecr describe-repositories --repository-names ${ECR_REPO} --region ${REGION} &> /dev/null; then
        IMAGE_COUNT=$(aws ecr list-images \
            --repository-name ${ECR_REPO} \
            --query 'length(imageIds)' \
            --output text \
            --region ${REGION} 2>/dev/null || echo "0")
        echo "  ECR Repository: ${ECR_REPO} (${IMAGE_COUNT} images)"
    else
        echo "  ECR Repository: Not found"
    fi
    
    echo ""
}

# Confirm deletion
confirm_deletion() {
    show_resources
    
    if [ "$1" = "--force" ] || [ "$1" = "-f" ]; then
        log "Force flag detected, skipping confirmation"
        return 0
    fi
    
    warn "This will delete ALL AWS resources for the Swagger Docs MCP project!"
    warn "This action cannot be undone."
    echo ""
    read -p "Are you sure you want to continue? (yes/no): " confirmation
    
    if [ "$confirmation" != "yes" ]; then
        log "Deletion cancelled"
        exit 0
    fi
}

# Main destroy function
case "$1" in
    "stack")
        check_prerequisites
        show_resources
        confirm_deletion "$2"
        delete_stack
        ;;
    "lambda")
        check_prerequisites
        show_resources
        confirm_deletion "$2"
        delete_lambda
        ;;
    "ecr")
        check_prerequisites
        show_resources
        confirm_deletion "$2"
        delete_ecr
        ;;
    "docker")
        cleanup_docker
        ;;
    "all")
        check_prerequisites
        show_resources
        confirm_deletion "$2"
        delete_lambda
        delete_stack
        delete_ecr
        cleanup_docker
        log "All resources deleted successfully!"
        ;;
    "show")
        check_prerequisites
        show_resources
        ;;
    *)
        echo "Usage: $0 {stack|lambda|ecr|docker|all|show} [--force|-f]"
        echo ""
        echo "Commands:"
        echo "  stack  - Delete CloudFormation stack (handles ROLLBACK_COMPLETE state)"
        echo "  lambda - Delete Lambda function"
        echo "  ecr    - Delete ECR repository and all images"
        echo "  docker - Clean up local Docker images"
        echo "  all    - Delete all AWS resources and clean up Docker"
        echo "  show   - Show current resources without deleting"
        echo ""
        echo "Options:"
        echo "  --force, -f  - Skip confirmation prompt"
        echo ""
        echo "Examples:"
        echo "  $0 stack              # Delete just the CloudFormation stack"
        echo "  $0 all --force        # Delete everything without confirmation"
        echo "  $0 show               # Show current resources"
        echo ""
        echo "Prerequisites:"
        echo "  - AWS CLI configured with appropriate permissions"
        exit 1
        ;;
esac