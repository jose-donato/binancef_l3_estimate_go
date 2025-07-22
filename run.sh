#!/bin/bash

# Build and run the L3 Order Book Visualizer
# Usage: ./run.sh [symbol]
# Example: ./run.sh ETHUSDT

SYMBOL=${1:-ETHUSDT}

echo "🚀 Starting L3 Order Book Visualizer for $SYMBOL"
echo "📊 Access the web interface at: http://localhost:8080"
echo "🔄 Use Ctrl+C to stop the server"
echo ""

# Build all Go files and run
go run *.go $(echo $SYMBOL | tr '[:upper:]' '[:lower:]')