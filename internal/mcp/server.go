package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/achmadsalmanr/mcp-server/internal/config"
	"github.com/achmadsalmanr/mcp-server/internal/review"
	"github.com/achmadsalmanr/mcp-server/internal/testrail"
)

const protocolVersion = "2025-06-18"

type Server struct {
	review   *review.Service
	testrail *testrail.Client
	statuses []config.ProviderStatus
	warnings []string
	tools    []Tool
}

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
	Handler     ToolHandler    `json:"-"`
}

type ToolHandler func(context.Context, json.RawMessage) (any, error)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewServer(reviewSvc *review.Service, tr *testrail.Client, statuses []config.ProviderStatus, warnings []string) *Server {
	s := &Server{review: reviewSvc, testrail: tr, statuses: statuses, warnings: warnings}
	s.registerTools()
	return s
}

func (s *Server) RunStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := strings.TrimPrefix(strings.TrimSpace(scanner.Text()), "\ufeff")
		if line == "" {
			continue
		}
		response, shouldRespond := s.handle(ctx, []byte(line))
		if !shouldRespond {
			continue
		}
		b, err := json.Marshal(response)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, string(b)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, raw []byte) (rpcResponse, bool) {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return errorResponse(nil, -32700, "parse error: "+err.Error()), true
	}
	if len(req.ID) == 0 {
		if req.Method == "notifications/initialized" || strings.HasPrefix(req.Method, "notifications/") {
			return rpcResponse{}, false
		}
	}
	switch req.Method {
	case "initialize":
		return resultResponse(req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    "multi-git-testrail-mcp",
				"version": "0.1.0",
			},
		}), len(req.ID) != 0
	case "ping":
		return resultResponse(req.ID, map[string]any{}), len(req.ID) != 0
	case "tools/list":
		return resultResponse(req.ID, map[string]any{"tools": s.toolDefs()}), len(req.ID) != 0
	case "tools/call":
		result, err := s.callTool(ctx, req.Params)
		if err != nil {
			return resultResponse(req.ID, toolError(err.Error())), len(req.ID) != 0
		}
		return resultResponse(req.ID, toolSuccess(result)), len(req.ID) != 0
	default:
		return errorResponse(req.ID, -32601, "method not found: "+req.Method), len(req.ID) != 0
	}
}

func (s *Server) toolDefs() []Tool {
	out := make([]Tool, 0, len(s.tools))
	for _, tool := range s.tools {
		tool.Handler = nil
		out = append(out, tool)
	}
	return out
}

func (s *Server) callTool(ctx context.Context, params json.RawMessage) (any, error) {
	var input struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("invalid tools/call params: %w", err)
	}
	if len(input.Arguments) == 0 || string(input.Arguments) == "null" {
		input.Arguments = []byte("{}")
	}
	for _, tool := range s.tools {
		if tool.Name == input.Name {
			return tool.Handler(ctx, input.Arguments)
		}
	}
	return nil, fmt.Errorf("unknown tool %q", input.Name)
}

func resultResponse(id json.RawMessage, result any) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: idOrNull(id), Result: result}
}

func errorResponse(id json.RawMessage, code int, message string) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: idOrNull(id), Error: &rpcError{Code: code, Message: message}}
}

func idOrNull(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return json.RawMessage("null")
	}
	return id
}

func toolSuccess(value any) map[string]any {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		b = []byte(fmt.Sprintf("%v", value))
	}
	return map[string]any{
		"content":           []map[string]string{{"type": "text", "text": string(b)}},
		"structuredContent": value,
	}
}

func toolError(message string) map[string]any {
	return map[string]any{
		"isError": true,
		"content": []map[string]string{{"type": "text", "text": message}},
	}
}

func readOnly() map[string]any {
	return map[string]any{"readOnlyHint": true}
}

func writeTool() map[string]any {
	return map[string]any{"readOnlyHint": false, "destructiveHint": false}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func stringProp(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func intProp(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

func boolProp(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func enumProp(description string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": description, "enum": values}
}

func decodeArgs[T any](args json.RawMessage) (T, error) {
	var input T
	if err := json.Unmarshal(args, &input); err != nil {
		return input, err
	}
	return input, nil
}
