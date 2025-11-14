#!/bin/bash

# Quick start script for ecommerce-express example

set -e

echo "🚀 IngestKit E-Commerce Express Example - Quick Start"
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
if [ ! -f "ingestkit/client.ts" ]; then
    echo "→ Generating IngestKit client..."
    ../../bin/ingestkit generate
    echo "✓ Client generated"
    echo
else
    echo "✓ IngestKit client already generated"
    echo
fi

# Check if node_modules exists
if [ ! -d "node_modules" ]; then
    echo "→ Installing dependencies..."
    npm install
    echo "✓ Dependencies installed"
    echo
else
    echo "✓ Dependencies already installed"
    echo
fi

# Copy .env if needed
if [ ! -f ".env" ]; then
    cp .env.example .env
    echo "✓ Created .env from .env.example"
    echo
fi

echo "=========================================="
echo "✅ Setup complete! Starting Express app..."
echo "=========================================="
echo
echo "The app will run on http://localhost:3000"
echo
echo "To test the app, run in another terminal:"
echo "  cd examples/ecommerce-express"
echo "  ./test-flow.sh"
echo
echo "Press Ctrl+C to stop the server"
echo
echo "=========================================="
echo

# Start the Express app
npm start
