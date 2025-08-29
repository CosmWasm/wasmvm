#!/bin/bash

echo "🚀 Testing WasmVM RPC Server Connection"

# Start the server in the background
echo "Starting server on port 50051..."
cargo run --release &
SERVER_PID=$!

# Wait for server to start
echo "Waiting for server to start..."
sleep 3

# Test connection using grpcurl (if available) or nc
echo "Testing connection..."
if command -v grpcurl &>/dev/null; then
  echo "Using grpcurl to test connection..."
  grpcurl -plaintext localhost:50051 list
else
  echo "Using nc to test port connectivity..."
  nc -z localhost 50051
  if [ $? -eq 0 ]; then
    echo "✅ Port 50051 is open and accepting connections"
  else
    echo "❌ Port 50051 is not accessible"
  fi
fi

# Clean up
echo "Stopping server..."
kill $SERVER_PID
wait $SERVER_PID 2>/dev/null

echo "✅ Connection test complete"
