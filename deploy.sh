#!/bin/bash
set -e

APP_NAME="ai-exam-simulator"
IMAGE_NAME="${APP_NAME}:latest"
PREV_IMAGE_NAME="${APP_NAME}:previous"

# 1. Load config
if [ ! -f "deploy.env" ]; then
    echo "Creating deploy.env template. Please fill it out and run this script again."
    cat > deploy.env << 'EOF'
VPS_USER="ec2-user"
VPS_HOST="your-lightsail-ip"
SSH_KEY="~/.ssh/your-key.pem"
DOMAIN="your-domain.com"
EOF
    exit 1
fi
source deploy.env

SSH_CMD="ssh -i ${SSH_KEY} -o StrictHostKeyChecking=accept-new ${VPS_USER}@${VPS_HOST}"

# Setup mode
if [ "$1" == "setup" ]; then
    echo "🛠️ Running one-time setup on VPS..."
    # Copy the env file
    scp -i ${SSH_KEY} .env ${VPS_USER}@${VPS_HOST}:/home/${VPS_USER}/.env

    $SSH_CMD << EOF
    sudo dnf update -y
    sudo dnf install -y docker
    sudo systemctl enable --now docker
    sudo usermod -aG docker ${VPS_USER}
    
    # 1GB Swap (if not exists)
    if [ ! -f /swapfile ]; then
        sudo fallocate -l 1G /swapfile || sudo dd if=/dev/zero of=/swapfile bs=1M count=1024
        sudo chmod 600 /swapfile
        sudo mkswap /swapfile
        sudo swapon /swapfile
        echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
    fi

    # Ensure network
    docker network create web || true

    # Start Caddy
    cat > Caddyfile << 'CADDYEOF'
${DOMAIN} {
    reverse_proxy 127.0.0.1:8080
}
CADDYEOF
    
    docker stop caddy || true
    docker rm caddy || true
    docker run -d --name caddy \
        --restart unless-stopped \
        --network host \
        -v \$PWD/Caddyfile:/etc/caddy/Caddyfile \
        -v caddy_data:/data \
        caddy:alpine
EOF
    echo "✅ Setup complete. Caddy is running."
    exit 0
fi

if [ "$1" == "logs" ]; then
    $SSH_CMD "docker logs -f ${APP_NAME}"
    exit 0
fi

if [ "$1" == "rollback" ]; then
    echo "⏪ Rolling back to previous image..."
    $SSH_CMD << EOF
    docker stop ${APP_NAME} || true
    docker rm ${APP_NAME} || true
    docker run -d --name ${APP_NAME} \
        --restart unless-stopped \
        --memory=640m \
        -p 127.0.0.1:8080:8080 \
        --env-file .env \
        ${PREV_IMAGE_NAME}
EOF
    echo "✅ Rollback complete."
    exit 0
fi

# Default deployment
echo "📦 Building Docker image for linux/amd64 (Lightsail)..."
# We don't have go installed on the Mac, and Docker might not be able to build
# natively if Docker Desktop isn't running. But the user will run this when Docker is running.
docker build --platform linux/amd64 -t ${IMAGE_NAME} .

echo "🚢 Shipping image to VPS via SSH..."
# Tag previous on remote
$SSH_CMD "docker tag ${IMAGE_NAME} ${PREV_IMAGE_NAME} 2>/dev/null || true"

# Export, compress, pipe over SSH, and load
docker save ${IMAGE_NAME} | gzip | $SSH_CMD "gunzip | docker load"

echo "🔄 Restarting container..."
$SSH_CMD << EOF
    docker stop ${APP_NAME} || true
    docker rm ${APP_NAME} || true
    docker run -d --name ${APP_NAME} \
        --restart unless-stopped \
        --memory=640m \
        -p 127.0.0.1:8080:8080 \
        --env-file .env \
        ${IMAGE_NAME}
    
    # Prune old images to save disk space
    docker image prune -f
EOF

echo "✅ Success! Container is running."
echo "Wait a few seconds, then check logs with: ./deploy.sh logs"
