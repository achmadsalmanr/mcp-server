package install

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunInstallUsesDefaultServerName(t *testing.T) {
	var stdout, stderr strings.Builder
	code := RunInstall(nil, Options{
		Stdout:     &stdout,
		Stderr:     &stderr,
		Executable: `C:\tools\mcp-server.exe`,
	})
	if code != 0 {
		t.Fatalf("RunInstall() code = %d, want 0; stderr = %q", code, stderr.String())
	}

	var output struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &output); err != nil {
		t.Fatalf("generated config is not JSON: %v", err)
	}
	if _, ok := output.MCPServers["multi-mcp"]; !ok {
		t.Fatalf("mcpServers = %v, want multi-mcp key", output.MCPServers)
	}
}
