package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/achmadsalmanr/mcp-server/internal/review"
)

func TestInitializeAndListTools(t *testing.T) {
	server := NewServer(review.NewService(), nil, nil, nil)
	input := strings.NewReader("\ufeff" + `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
`)
	var out strings.Builder
	if err := server.RunStdio(context.Background(), input, &out); err != nil {
		t.Fatalf("RunStdio() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("response lines = %d, want 2: %q", len(lines), out.String())
	}
	var initResp map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &initResp); err != nil {
		t.Fatalf("initialize response is not JSON: %v", err)
	}
	result := initResp["result"].(map[string]any)
	if result["protocolVersion"] != protocolVersion {
		t.Fatalf("protocolVersion = %v, want %s", result["protocolVersion"], protocolVersion)
	}
	var toolsResp map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &toolsResp); err != nil {
		t.Fatalf("tools response is not JSON: %v", err)
	}
	toolsResult := toolsResp["result"].(map[string]any)
	tools := toolsResult["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("tools/list returned no tools")
	}
}
