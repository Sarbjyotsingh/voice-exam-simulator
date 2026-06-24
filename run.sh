#!/bin/bash

# Configuration
PORT=${PORT:-8000}
HOST=${HOST:-127.0.0.1}

# Exit on any error
set -e

echo "🚀 Starting local development server..."

# 1. Check if Python is available
if ! command -v python3 &> /dev/null; then
    echo "❌ Python 3 is not installed. Please install Python 3.11 or later."
    exit 1
fi

# 2. Create virtual environment if it doesn't exist
if [ ! -d "venv" ]; then
    echo "📦 Creating virtual environment..."
    python3 -m venv venv
fi

# 3. Activate virtual environment
echo "✨ Activating virtual environment..."
source venv/bin/activate

# 4. Install/upgrade dependencies
echo "📥 Installing dependencies..."
pip install -q --upgrade pip
pip install -q -r requirements.txt

# 5. Run the app
echo "🎯 Starting Uvicorn server on http://${HOST}:${PORT}"
echo "📝 API Docs available at http://${HOST}:${PORT}/docs"
echo "💬 WebSocket endpoint: ws://${HOST}:${PORT}/exam"
echo ""
echo "Press Ctrl+C to stop the server"
echo ""

uvicorn app.main:app --host ${HOST} --port ${PORT} --reload
