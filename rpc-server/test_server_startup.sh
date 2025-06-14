#!/bin/bash

echo "🚀 Testing WasmVM RPC Server Startup Timing"

# Start the server and capture output
echo "Starting server on port 50051..."
cargo run --release 2>&1 | tee server.log &
SERVER_PID=$!

# Monitor server startup
echo "Monitoring server startup..."
START_TIME=$(date +%s)

# Wait for server to be ready (look for the ready message)
while ! grep -q "WasmVM gRPC server ready and listening" server.log 2>/dev/null; do
  CURRENT_TIME=$(date +%s)
  ELAPSED=$((CURRENT_TIME - START_TIME))

  if [ $ELAPSED -gt 10 ]; then
    echo "❌ Server failed to start within 10 seconds"
    kill $SERVER_PID 2>/dev/null
    cat server.log
    exit 1
  fi

  echo "⏳ Waiting for server... ($ELAPSED seconds)"
  sleep 0.5
done

END_TIME=$(date +%s)
STARTUP_TIME=$((END_TIME - START_TIME))

echo "✅ Server started successfully in $STARTUP_TIME seconds"

# Test connection
echo "Testing connection..."
nc -z localhost 50051
if [ $? -eq 0 ]; then
  echo "✅ Port 50051 is open and accepting connections"
else
  echo "❌ Port 50051 is not accessible"
fi

# Check for cache initialization timing
echo ""
echo "📊 Cache initialization timing:"
grep "init_cache took" server.log || echo "No cache timing info found"

# Clean up
echo ""
echo "Stopping server..."
kill $SERVER_PID
wait $SERVER_PID 2>/dev/null

echo "✅ Test complete"
echo ""
echo "📋 Server startup log:"
cat server.log
