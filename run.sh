#!/bin/bash
set -e

PORT=${PORT:-8000}
HOST=${HOST:-127.0.0.1}

echo "🚀 Starting local development server..."

if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

export PORT=$PORT

echo "🎯 Starting server on http://${HOST}:${PORT}"
echo "📝 API Docs: not generated automatically in Go, see code"
echo "💬 WebSocket endpoint: ws://${HOST}:${PORT}/exam"
echo ""

if command -v go &> /dev/null; then
    echo "✨ Using local Go installation..."
    go run .
else
    echo "🐳 Go not found locally. Falling back to Docker..."
    ENV_ARG=""
    if [ -f .env ]; then
        ENV_ARG="--env-file .env"
    fi
    docker run --rm -it \
        -v "$PWD":/src \
        -w /src \
        -p 0.0.0.0:${PORT}:${PORT} \
        $ENV_ARG \
        -e PORT=${PORT} \
        golang:1.24-alpine \
        go run .
fi
