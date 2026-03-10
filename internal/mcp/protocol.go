// Package mcp implements a stdio MCP server for cubit.
package mcp

import "encoding/json"

// JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolDef is an MCP tool definition returned by tools/list.
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// ToolResult is the result of an MCP tools/call invocation.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is a text content block in MCP tool results.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// textResult returns a single text block result.
func textResult(s string) *ToolResult {
	return &ToolResult{Content: []ContentBlock{{Type: "text", Text: s}}}
}

// errorResult returns a tool-level error result.
func errorResult(s string) *ToolResult {
	return &ToolResult{Content: []ContentBlock{{Type: "text", Text: s}}, IsError: true}
}
