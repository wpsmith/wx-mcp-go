#!/bin/bash

# AWS Deployment Script for Swagger Docs MCP
set -e

REGION="us-east-1"
STACK_NAME="swagger-docs-mcp"
ECR_REPO="swagger-docs-mcp"
WEATHER_API_KEY="${WX_MCP_API_KEY}"

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
    
    if ! command -v docker &> /dev/null; then
        error "Docker not found. Please install it first."
        exit 1
    fi
    
    if [ -z "$WEATHER_API_KEY" ]; then
        error "WX_MCP_API_KEY environment variable not set"
        exit 1
    fi
    
    # Check AWS credentials
    if ! aws sts get-caller-identity &> /dev/null; then
        error "AWS credentials not configured or invalid"
        exit 1
    fi
    
    log "Prerequisites check passed"
}

# Create ECR repository
create_ecr_repo() {
    log "Creating ECR repository..."
    
    ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
    ECR_URI="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${ECR_REPO}"
    
    # Create repository if it doesn't exist
    if ! aws ecr describe-repositories --repository-names ${ECR_REPO} --region ${REGION} &> /dev/null; then
        aws ecr create-repository \
            --repository-name ${ECR_REPO} \
            --region ${REGION} \
            --image-scanning-configuration scanOnPush=true
        log "ECR repository created: ${ECR_URI}"
    else
        log "ECR repository already exists: ${ECR_URI}"
    fi
}

# Build and push Docker image
build_and_push() {
    log "Building and pushing Docker image..."
    
    ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
    ECR_URI="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${ECR_REPO}"
    
    # Login to ECR
    aws ecr get-login-password --region ${REGION} | docker login --username AWS --password-stdin ${ECR_URI}
    
    # Build image for linux/amd64 platform (required for ECS Fargate)
    cd "$(dirname "$0")/.."
    docker build --platform linux/amd64 -t ${ECR_REPO}:latest .
    docker tag ${ECR_REPO}:latest ${ECR_URI}:latest
    
    # Push image
    docker push ${ECR_URI}:latest
    
    log "Image pushed: ${ECR_URI}:latest"
}

# Deploy CloudFormation stack
deploy_stack() {
    log "Deploying CloudFormation stack..."
    
    # Get default VPC and subnets
    VPC_ID=$(aws ec2 describe-vpcs --filters "Name=is-default,Values=true" --query 'Vpcs[0].VpcId' --output text --region ${REGION})
    SUBNET_IDS=$(aws ec2 describe-subnets --filters "Name=vpc-id,Values=${VPC_ID}" --query 'Subnets[].SubnetId' --output text --region ${REGION} | tr '\t' ',')
    
    if [ "$VPC_ID" = "None" ] || [ -z "$SUBNET_IDS" ]; then
        error "No default VPC found. Please specify VPC and subnet IDs manually."
        exit 1
    fi
    
    log "Using VPC: ${VPC_ID}"
    log "Using Subnets: ${SUBNET_IDS}"
    
    # Deploy stack
    aws cloudformation deploy \
        --template-file aws/cloudformation-template.yaml \
        --stack-name ${STACK_NAME} \
        --parameter-overrides \
            VpcId=${VPC_ID} \
            SubnetIds=${SUBNET_IDS} \
            WeatherApiKey=${WEATHER_API_KEY} \
        --capabilities CAPABILITY_NAMED_IAM \
        --tags \
            Project=Wx_API_MCP \
            Environment=Production \
            Application=SwaggerDocsMCP \
            Owner=WeatherAPI \
        --region ${REGION}
    
    # Get outputs
    LOAD_BALANCER_URL=$(aws cloudformation describe-stacks \
        --stack-name ${STACK_NAME} \
        --query 'Stacks[0].Outputs[?OutputKey==`LoadBalancerURL`].OutputValue' \
        --output text \
        --region ${REGION})
    
    log "Deployment complete!"
    log "Load Balancer URL: ${LOAD_BALANCER_URL}"
    log "Health Check: ${LOAD_BALANCER_URL}/health"
    log "Tools Endpoint: ${LOAD_BALANCER_URL}/tools"
}

# Deploy Lambda function
deploy_lambda() {
    log "Deploying Lambda MCP proxy..."
    
    ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
    ECR_URI="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${ECR_REPO}"
    
    # Get ALB URL from CloudFormation output
    SWAGGER_DOCS_URL=$(aws cloudformation describe-stacks \
        --stack-name ${STACK_NAME} \
        --query 'Stacks[0].Outputs[?OutputKey==`LoadBalancerURL`].OutputValue' \
        --output text \
        --region ${REGION})
    
    # Create Lambda function
    cd aws
    zip -r lambda-mcp-proxy.zip lambda-mcp-proxy.js
    
    # Create or update Lambda function
    if aws lambda get-function --function-name swagger-docs-mcp-proxy --region ${REGION} &> /dev/null; then
        aws lambda update-function-code \
            --function-name swagger-docs-mcp-proxy \
            --zip-file fileb://lambda-mcp-proxy.zip \
            --region ${REGION}
    else
        aws lambda create-function \
            --function-name swagger-docs-mcp-proxy \
            --runtime nodejs18.x \
            --role $(aws iam get-role --role-name lambda-execution-role --query 'Role.Arn' --output text 2>/dev/null || echo "arn:aws:iam::${ACCOUNT_ID}:role/lambda-execution-role") \
            --handler lambda-mcp-proxy.handler \
            --zip-file fileb://lambda-mcp-proxy.zip \
            --timeout 300 \
            --memory-size 512 \
            --environment Variables="{SWAGGER_DOCS_URL=${SWAGGER_DOCS_URL},WX_MCP_API_KEY=${WEATHER_API_KEY}}" \
            --region ${REGION}
    fi
    
    rm lambda-mcp-proxy.zip
    cd ..
    
    log "Lambda function deployed"
}

# Main deployment
case "$1" in
    "ecr")
        check_prerequisites
        create_ecr_repo
        ;;
    "build")
        check_prerequisites
        create_ecr_repo
        build_and_push
        ;;
    "stack")
        check_prerequisites
        deploy_stack
        ;;
    "lambda")
        check_prerequisites
        deploy_lambda
        ;;
    "all")
        check_prerequisites
        create_ecr_repo
        build_and_push
        deploy_stack
        deploy_lambda
        ;;
    *)
        echo "Usage: $0 {ecr|build|stack|lambda|all}"
        echo ""
        echo "Commands:"
        echo "  ecr    - Create ECR repository"
        echo "  build  - Build and push Docker image"
        echo "  stack  - Deploy CloudFormation stack"
        echo "  lambda - Deploy Lambda MCP proxy"
        echo "  all    - Run all steps"
        echo ""
        echo "Prerequisites:"
        echo "  - AWS CLI configured with appropriate permissions"
        echo "  - Docker installed and running"
        echo "  - WX_MCP_API_KEY environment variable set"
        exit 1
        ;;
esac