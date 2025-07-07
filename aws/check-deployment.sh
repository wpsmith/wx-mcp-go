#!/bin/bash

# Check AWS deployment status
set -e

REGION="us-east-1"
STACK_NAME="swagger-docs-mcp"

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

check_stack_status() {
    log "Checking CloudFormation stack status..."
    
    STATUS=$(aws cloudformation describe-stacks \
        --stack-name ${STACK_NAME} \
        --region ${REGION} \
        --query 'Stacks[0].StackStatus' \
        --output text 2>/dev/null || echo "NOT_FOUND")
    
    echo "Stack Status: $STATUS"
    
    case $STATUS in
        "CREATE_COMPLETE")
            log "✅ Stack deployment successful!"
            show_outputs
            ;;
        "CREATE_IN_PROGRESS")
            warn "⏳ Stack is still being created. Please wait..."
            show_recent_events
            ;;
        "CREATE_FAILED"|"ROLLBACK_COMPLETE"|"ROLLBACK_IN_PROGRESS")
            error "❌ Stack deployment failed!"
            show_failure_events
            ;;
        "NOT_FOUND")
            warn "📭 Stack not found. Run deployment first."
            ;;
        *)
            warn "🔄 Stack status: $STATUS"
            ;;
    esac
}

show_outputs() {
    log "Stack outputs:"
    aws cloudformation describe-stacks \
        --stack-name ${STACK_NAME} \
        --region ${REGION} \
        --query 'Stacks[0].Outputs[].{Key:OutputKey,Value:OutputValue}' \
        --output table
    
    # Show ALB URL specifically
    ALB_URL=$(aws cloudformation describe-stacks \
        --stack-name ${STACK_NAME} \
        --region ${REGION} \
        --query 'Stacks[0].Outputs[?OutputKey==`LoadBalancerURL`].OutputValue' \
        --output text)
    
    if [ "$ALB_URL" != "None" ] && [ -n "$ALB_URL" ]; then
        log "🌐 Application Load Balancer URL: $ALB_URL"
        log "🏥 Health Check: $ALB_URL/health"
        log "🔧 Tools Endpoint: $ALB_URL/tools"
        log "⚙️  Config Endpoint: $ALB_URL/config"
        
        # Test health endpoint
        log "Testing health endpoint..."
        if curl -s -f "$ALB_URL/health" > /dev/null; then
            log "✅ Health check passed!"
        else
            warn "⚠️  Health check failed or service not ready yet"
        fi
    fi
}

show_recent_events() {
    log "Recent events:"
    aws cloudformation describe-stack-events \
        --stack-name ${STACK_NAME} \
        --region ${REGION} \
        --max-items 5 \
        --query 'StackEvents[].{Time:Timestamp,Status:ResourceStatus,Type:ResourceType,Reason:ResourceStatusReason}' \
        --output table
}

show_failure_events() {
    error "Failure events:"
    aws cloudformation describe-stack-events \
        --stack-name ${STACK_NAME} \
        --region ${REGION} \
        --query 'StackEvents[?contains(ResourceStatus, `FAILED`)].{Time:Timestamp,Resource:LogicalResourceId,Status:ResourceStatus,Reason:ResourceStatusReason}' \
        --output table
}

check_ecs_service() {
    log "Checking ECS service status..."
    
    CLUSTER_NAME="swagger-docs-mcp-cluster"
    SERVICE_NAME="swagger-docs-mcp-service"
    
    if aws ecs describe-services \
        --cluster ${CLUSTER_NAME} \
        --services ${SERVICE_NAME} \
        --region ${REGION} &> /dev/null; then
        
        aws ecs describe-services \
            --cluster ${CLUSTER_NAME} \
            --services ${SERVICE_NAME} \
            --region ${REGION} \
            --query 'services[0].{Status:status,Running:runningCount,Desired:desiredCount,Pending:pendingCount}' \
            --output table
    else
        warn "ECS service not found or not accessible"
    fi
}

# Main execution
case "$1" in
    "status"|"")
        check_stack_status
        ;;
    "ecs")
        check_ecs_service
        ;;
    "wait")
        log "Waiting for stack to complete..."
        aws cloudformation wait stack-create-complete \
            --stack-name ${STACK_NAME} \
            --region ${REGION}
        check_stack_status
        ;;
    "outputs")
        show_outputs
        ;;
    *)
        echo "Usage: $0 {status|ecs|wait|outputs}"
        echo ""
        echo "Commands:"
        echo "  status  - Check stack status (default)"
        echo "  ecs     - Check ECS service status"
        echo "  wait    - Wait for stack completion"
        echo "  outputs - Show stack outputs"
        exit 1
        ;;
esac