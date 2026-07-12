package install

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/achmadsalmanr/mcp-server/internal/config"
)

type generatedServer struct {
	Command string            `json:"command"`
	Env     map[string]string `json:"env"`
}

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
		MCPServers map[string]generatedServer `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &output); err != nil {
		t.Fatalf("generated config is not JSON: %v", err)
	}
	server, ok := output.MCPServers["multi-mcp"]
	if !ok {
		t.Fatalf("mcpServers = %v, want multi-mcp key", output.MCPServers)
	}
	if server.Command != `C:\tools\mcp-server.exe` {
		t.Fatalf("command = %q, want configured executable", server.Command)
	}
	if server.Env != nil {
		t.Fatalf("env = %v, want environment inheritance by default", server.Env)
	}
}

func TestRunInstallCanEmitEnvPlaceholders(t *testing.T) {
	server, code, stderr := runInstall(t, []string{"--env-placeholders"}, config.Config{})
	if code != 0 {
		t.Fatalf("RunInstall() code = %d, want 0; stderr = %q", code, stderr)
	}
	if server.Env["GITHUB_TOKEN"] != "${GITHUB_TOKEN}" {
		t.Fatalf("GITHUB_TOKEN = %q, want placeholder", server.Env["GITHUB_TOKEN"])
	}
	if server.Env["FIGMA_ACCESS_TOKEN"] != "${FIGMA_ACCESS_TOKEN}" {
		t.Fatalf("FIGMA_ACCESS_TOKEN = %q, want placeholder", server.Env["FIGMA_ACCESS_TOKEN"])
	}
}

func TestRunInstallCanIncludeSecrets(t *testing.T) {
	cfg := config.Config{
		GitHub: config.GitHubConfig{Token: "github-token"},
		Figma:  config.FigmaConfig{Token: "figma-token"},
	}
	server, code, stderr := runInstall(t, []string{"--include-secrets"}, cfg)
	if code != 0 {
		t.Fatalf("RunInstall() code = %d, want 0; stderr = %q", code, stderr)
	}
	if server.Env["GITHUB_TOKEN"] != "github-token" {
		t.Fatalf("GITHUB_TOKEN = %q, want configured value", server.Env["GITHUB_TOKEN"])
	}
	if server.Env["FIGMA_ACCESS_TOKEN"] != "figma-token" {
		t.Fatalf("FIGMA_ACCESS_TOKEN = %q, want configured value", server.Env["FIGMA_ACCESS_TOKEN"])
	}
}

func TestRunInstallRejectsConflictingEnvModes(t *testing.T) {
	_, code, stderr := runInstall(t, []string{"--env-placeholders", "--include-secrets"}, config.Config{})
	if code != 2 {
		t.Fatalf("RunInstall() code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "cannot be used together") {
		t.Fatalf("stderr = %q, want conflicting flags message", stderr)
	}
}

func runInstall(t *testing.T, args []string, cfg config.Config) (generatedServer, int, string) {
	t.Helper()
	var stdout, stderr strings.Builder
	code := RunInstall(args, Options{
		Stdout:     &stdout,
		Stderr:     &stderr,
		Executable: `C:\tools\mcp-server.exe`,
		Config:     cfg,
	})
	if stdout.Len() == 0 {
		return generatedServer{}, code, stderr.String()
	}
	var output struct {
		MCPServers map[string]generatedServer `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &output); err != nil {
		t.Fatalf("generated config is not JSON: %v", err)
	}
	return output.MCPServers["multi-mcp"], code, stderr.String()
}
