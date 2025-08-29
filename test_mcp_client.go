package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func main() {
	fmt.Println("🧪 MCP Kubernetes Server Test Client")
	fmt.Println("====================================")

	// Build the server first
	fmt.Println("Building MCP server...")
	cmd := exec.Command("go", "build", "-o", "bin/mcp-server", "./cmd/server")
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to build server: %v", err)
	}
	fmt.Println("✅ Server built successfully")

	// Test cases
	tests := []struct {
		name        string
		request     JSONRPCRequest
		description string
	}{
		{
			name: "initialize",
			request: JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "initialize",
				Params:  map[string]interface{}{},
			},
			description: "Initialize MCP server",
		},
		{
			name: "tools_list",
			request: JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      2,
				Method:  "tools/list",
				Params:  map[string]interface{}{},
			},
			description: "List available tools",
		},
		{
			name: "echo_test",
			request: JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      3,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name":      "echo",
					"arguments": map[string]interface{}{"text": "Hello from test client!"},
				},
			},
			description: "Test echo tool (returns string)",
		},
		{
			name: "cluster_health",
			request: JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      4,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name":      "cluster_health",
					"arguments": map[string]interface{}{},
				},
			},
			description: "Test cluster_health tool (returns structured object)",
		},
		{
			name: "pods_list",
			request: JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      5,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name": "pods_list",
					"arguments": map[string]interface{}{
						"namespace": "default",
						"limit":     3,
					},
				},
			},
			description: "Test pods_list tool (returns structured array)",
		},
	}

	// Start MCP server
	fmt.Println("\n🚀 Starting MCP server...")
	serverCmd := exec.Command("./bin/mcp-server")
	serverStdin, err := serverCmd.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to create stdin pipe: %v", err)
	}
	serverStdout, err := serverCmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to create stdout pipe: %v", err)
	}
	serverStderr, err := serverCmd.StderrPipe()
	if err != nil {
		log.Fatalf("Failed to create stderr pipe: %v", err)
	}

	if err := serverCmd.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Read stderr in background to avoid blocking
	go func() {
		scanner := bufio.NewScanner(serverStderr)
		for scanner.Scan() {
			fmt.Printf("📝 SERVER LOG: %s\n", scanner.Text())
		}
	}()

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Send requests and read responses
	encoder := json.NewEncoder(serverStdin)
	decoder := json.NewDecoder(serverStdout)

	for _, test := range tests {
		fmt.Printf("\n🔧 Testing: %s\n", test.description)

		// Send request
		if err := encoder.Encode(test.request); err != nil {
			fmt.Printf("❌ Failed to send request: %v\n", err)
			continue
		}

		// Read response
		var response JSONRPCResponse
		if err := decoder.Decode(&response); err != nil {
			fmt.Printf("❌ Failed to read response: %v\n", err)
			continue
		}

		// Pretty print response
		responseJSON, _ := json.MarshalIndent(response, "", "  ")
		fmt.Printf("📨 Response:\n%s\n", string(responseJSON))

		// Validate response
		if response.JSONRPC != "2.0" {
			fmt.Printf("❌ Invalid JSON-RPC version: %s\n", response.JSONRPC)
			continue
		}

		if response.ID != test.request.ID {
			fmt.Printf("❌ ID mismatch: expected %d, got %d\n", test.request.ID, response.ID)
			continue
		}

		if response.Error != nil {
			fmt.Printf("⚠️  Server returned error: %s (code: %d)\n", response.Error.Message, response.Error.Code)
			continue
		}

		// Check if result is structured JSON (not wrapped in MCP content format)
		resultJSON, _ := json.Marshal(response.Result)
		if strings.Contains(string(resultJSON), `"content":`) {
			fmt.Printf("❌ Result still wrapped in MCP content format\n")
		} else {
			fmt.Printf("✅ Result returned as structured JSON directly\n")
		}
	}

	// Clean up
	serverStdin.Close()
	if err := serverCmd.Wait(); err != nil {
		fmt.Printf("Server exited with error: %v\n", err)
	}

	fmt.Println("\n🎉 Testing completed!")
	fmt.Println("All tools should return structured JSON objects/arrays in the 'result' field.")
}
