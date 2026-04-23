#!/bin/bash

# Configuration
AWS_REGION="eu-central-1"
AWS_ACCOUNT_ID="148706310667"
REPO_NAME="lingua-stories/ai-exam-simulator"
IMAGE_NAME="ai-exam-simulator"

# Full ECR URI
ECR_URL="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
FULL_IMAGE_URI="${ECR_URL}/${REPO_NAME}:latest"

# Exit on any error
set -e

echo "🚀 Starting deployment for ${REPO_NAME}..."

# 1. Login to ECR
echo "🔑 Logging in to Amazon ECR..."
aws ecr get-login-password --region ${AWS_REGION} | docker login --username AWS --password-stdin ${ECR_URL}

# 2. Build the image
# Note: --platform linux/amd64 ensures it works on AWS ECS even if you are on a Mac M1/M2
echo "📦 Building Docker image (linux/amd64)..."
docker build --platform linux/amd64 -t ${IMAGE_NAME} .

# 3. Tag the image
echo "🏷️ Tagging image..."
docker tag ${IMAGE_NAME}:latest ${FULL_IMAGE_URI}

# 4. Push the image
echo "☁️ Pushing image to ECR..."
docker push ${FULL_IMAGE_URI}

echo "✅ Success! Your image is now on AWS ECR."
echo "🔗 URI: ${FULL_IMAGE_URI}"
