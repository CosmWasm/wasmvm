#!/bin/bash

# Generate Go protobuf files from wasmvm.proto
set -e

echo "Generating Go protobuf files..."

# Install protoc-gen-go and protoc-gen-go-grpc if not already installed
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Create output directory
mkdir -p ../rpc

# Generate Go code
protoc \
  --go_out=../rpc \
  --go_opt=paths=source_relative \
  --go-grpc_out=../rpc \
  --go-grpc_opt=paths=source_relative \
  wasmvm.proto

echo "Go protobuf files generated successfully in ../rpc/" 