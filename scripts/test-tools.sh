#!/bin/bash

# MCP Kubernetes Server Tool Testing Script
# Tests all tools to verify they return structured JSON responses

set -e

BINARY="./bin/mcp-server"

if [ ! -f "$BINARY" ]; then
    echo "Error: MCP server binary not found at $BINARY"
    echo "Please build it first: go build -o bin/mcp-server ./cmd/server"
    exit 1
fi

echo "🧪 Testing MCP Kubernetes Server Tools"
echo "====================================="

# Function to test a tool
test_tool() {
    local tool_name="$1"
    local arguments="$2"
    local description="$3"

    echo ""
    echo "🔧 Testing: $description ($tool_name)"
    echo "Request: {\"name\":\"$tool_name\",\"arguments\":$arguments}"

    # Create JSON-RPC request
    local request="{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{}}
{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"$tool_name\",\"arguments\":$arguments}}"

    # Send request and capture response
    local response=$(printf "$request" | $BINARY 2>/dev/null | tail -n1)

    echo "Response: $response"

    # Check if response is valid JSON
    if echo "$response" | jq . >/dev/null 2>&1; then
        echo "✅ Valid JSON response"
    else
        echo "❌ Invalid JSON response"
    fi
}

# Test echo tool (simple string response)
test_tool "echo" "{\"text\":\"Hello, MCP!\"}" "Echo tool"

# Test cluster_health (structured object response)
test_tool "cluster_health" "{}" "Cluster health"

# Test pods_list (structured array response) - will fail without K8s, but test format
test_tool "pods_list" "{\"namespace\":\"default\",\"limit\":3}" "Pods list"

# Test resources_get (structured response)
test_tool "resources_get" "{\"group\":\"\",\"version\":\"v1\",\"kind\":\"Service\",\"namespace\":\"default\"}" "Resources get"

echo ""
echo "🎉 Tool testing completed!"
echo "Note: Some tools may return errors if Kubernetes client is not initialized or cluster is not accessible."
echo "The important thing is that responses are valid JSON in the 'result' field."
