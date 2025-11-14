#!/bin/bash

# Quick start script for blog-flask example

set -e

echo "🚀 IngestKit Blog Flask Example - Quick Start"
echo

# Check if IngestKit server is running
echo "→ Checking IngestKit API server..."
if ! curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo "❌ IngestKit API server is not running on port 8080"
    echo
    echo "Please start IngestKit first:"
    echo "  cd ../.."
    echo "  make up"
    echo "  make run-api"
    echo "  make run-consumer"
    echo
    exit 1
fi
echo "✓ IngestKit API server is running"
echo

# Check if client is generated
if [ ! -f "ingestkit/client.py" ]; then
    echo "→ Generating IngestKit client..."
    ../../bin/ingestkit generate
    echo "✓ Client generated"
    echo
else
    echo "✓ IngestKit client already generated"
    echo
fi

# Check if virtual environment exists
if [ ! -d "venv" ]; then
    echo "→ Creating virtual environment..."
    python3 -m venv venv
    echo "✓ Virtual environment created"
    echo
fi

# Activate virtual environment and install dependencies
echo "→ Installing dependencies..."
source venv/bin/activate
pip install -q -r requirements.txt
echo "✓ Dependencies installed"
echo

# Copy .env if needed
if [ ! -f ".env" ]; then
    cp .env.example .env
    echo "✓ Created .env from .env.example"
    echo
fi

echo "=========================================="
echo "✅ Setup complete! Starting Flask app..."
echo "=========================================="
echo
echo "The app will run on http://localhost:5000"
echo
echo "To test the app, run in another terminal:"
echo "  cd examples/blog-flask"
echo "  ./test-flow-simple.sh"
echo
echo "Press Ctrl+C to stop the server"
echo
echo "=========================================="
echo

# Start the Flask app
python app.py
