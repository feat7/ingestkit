#!/bin/bash

# Quick start script for blog-flask example
# This script assumes you're running from within the IngestKit repository

set -e

echo "IngestKit Blog Flask Example"
echo "============================="
echo

# Check if we're in the right directory
if [ ! -f "app.py" ]; then
    echo "[error] Please run this script from the examples/blog-flask directory"
    exit 1
fi

# Detect IngestKit CLI
INGESTKIT_CMD=""
if command -v ingestkit &> /dev/null; then
    INGESTKIT_CMD="ingestkit"
    echo "[ok] Using ingestkit from PATH"
elif [ -f "../../bin/ingestkit" ]; then
    INGESTKIT_CMD="../../bin/ingestkit"
    echo "[ok] Using local binary"
else
    echo "[info] IngestKit CLI not found. Building..."
    (cd ../.. && make build)
    INGESTKIT_CMD="../../bin/ingestkit"
    echo "[ok] CLI built"
fi
echo

# Check if IngestKit server is running
echo "[check] IngestKit API server..."
if ! curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo "[error] IngestKit API server is not running on port 8080"
    echo
    echo "Please start IngestKit first:"
    echo "  cd ../.."
    echo "  make up"
    echo "  make run-api"
    echo "  make run-consumer"
    echo
    exit 1
fi
echo "[ok] API server is running"
echo

# Generate client if not present
if [ ! -f "ingestkit/client.py" ]; then
    echo "[info] Generating IngestKit client..."
    $INGESTKIT_CMD generate --schema-url http://localhost:8080/schema
    echo "[ok] Client generated"
    echo
else
    echo "[ok] IngestKit client already generated"
    echo
fi

# Create virtual environment if needed
if [ ! -d "venv" ]; then
    echo "[info] Creating virtual environment..."
    python3 -m venv venv
    echo "[ok] Virtual environment created"
    echo
fi

# Activate and install dependencies
echo "[info] Installing dependencies..."
source venv/bin/activate
pip install -q -r requirements.txt
echo "[ok] Dependencies installed"
echo

# Copy .env if needed
if [ ! -f ".env" ]; then
    cp .env.example .env
    echo "[ok] Created .env from .env.example"
    echo
fi

echo "============================="
echo "Setup complete! Starting Flask app"
echo "============================="
echo
echo "App URL: http://localhost:5000"
echo
echo "To test, run in another terminal:"
echo "  ./test-flow-simple.sh"
echo
echo "Press Ctrl+C to stop"
echo

python app.py
