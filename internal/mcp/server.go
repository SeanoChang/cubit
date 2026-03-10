package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/SeanoChang/cubit/internal/config"
	"github.com/SeanoChang/cubit/internal/queue"
)

// Server is a stdio MCP server.
type Server struct {
	cfg   *config.Config
	q     *queue.Queue
	tools *Registry
}

// NewServer creates an MCP server backed by the given config and queue.
func NewServer(cfg *config.Config, q *queue.Queue) *Server {
	s := &Server{cfg: cfg, q: q}
	s.tools = NewRegistry(q, cfg)
	return s
}

// Run reads JSON-RPC requests from r and writes responses to w.
// Blocks until r is closed or an unrecoverable error occurs.
func (s *Server) Run(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			resp := Response{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: -32700, Message: "parse error"},
			}
			s.writeResponse(w, resp)
			continue
		}

		resp := s.handle(req)
		if resp != nil {
			s.writeResponse(w, *resp)
		}
	}
	return scanner.Err()
}

func (s *Server) handle(req Request) *Response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

func (s *Server) handleInitialize(req Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "cubit",
				"version": "0.5.0",
			},
		},
	}
}

func (s *Server) handleToolsList(req Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"tools": s.tools.Definitions(),
		},
	}
}

func (s *Server) handleToolsCall(req Request) *Response {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: "invalid params"},
		}
	}

	result := s.tools.Call(params.Name, params.Arguments)
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) writeResponse(w io.Writer, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("mcp: marshal error: %v", err)
		return
	}
	fmt.Fprintf(w, "%s\n", data)
}
