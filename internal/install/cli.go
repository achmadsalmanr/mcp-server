package install

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/achmadsalmanr/mcp-server/internal/appinfo"
	"github.com/achmadsalmanr/mcp-server/internal/config"
)

const DefaultServerName = appinfo.Name

type Options struct {
	Stdout     io.Writer
	Stderr     io.Writer
	Executable string
	Config     config.Config
	Warnings   []string
}

func RunInstall(args []string, opts Options) int {
	stdout, stderr := writers(opts)
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	target := fs.String("target", "generic", "target MCP client")
	serverName := fs.String("server-name", DefaultServerName, "MCP server name")
	command := fs.String("command", opts.Executable, "server command path")
	envPlaceholders := fs.Bool("env-placeholders", false, "include an env block with ${ENV_VAR} placeholders")
	includeSecrets := fs.Bool("include-secrets", false, "include an env block with current environment values")
	printConfig := fs.Bool("print-config", false, "print config JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *target != "generic" {
		fmt.Fprintln(stderr, "only --target generic is supported in this build")
		return 2
	}
	if *envPlaceholders && *includeSecrets {
		fmt.Fprintln(stderr, "--env-placeholders and --include-secrets cannot be used together")
		return 2
	}
	if *command == "" {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintf(stderr, "resolving executable path: %v\n", err)
			return 1
		}
		*command = exe
	}
	serverConfig := map[string]any{
		"command": *command,
	}
	if *envPlaceholders || *includeSecrets {
		serverConfig["env"] = envBlock(opts.Config, *includeSecrets)
	}
	out := map[string]any{
		"mcpServers": map[string]any{
			*serverName: serverConfig,
		},
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *printConfig || *target == "generic" {
		fmt.Fprintln(stdout, string(b))
	}
	return 0
}

func RunDoctor(args []string, opts Options) int {
	stdout, stderr := writers(opts)
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	report := map[string]any{
		"providers": opts.Config.Statuses(),
		"warnings":  opts.Warnings,
	}
	if *jsonOutput {
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, string(b))
		return 0
	}
	for _, status := range opts.Config.Statuses() {
		state := "configured"
		if !status.Configured {
			state = "missing " + fmt.Sprint(status.Missing)
		}
		fmt.Fprintf(stdout, "%s: %s\n", status.Name, state)
		for _, note := range status.Notes {
			fmt.Fprintf(stdout, "  note: %s\n", note)
		}
	}
	for _, warning := range opts.Warnings {
		fmt.Fprintf(stdout, "warning: %s\n", warning)
	}
	return 0
}

func envBlock(cfg config.Config, includeSecrets bool) map[string]string {
	env := map[string]string{
		"BITBUCKET_WORKSPACE": envValue("BITBUCKET_WORKSPACE", cfg.Bitbucket.Workspace, includeSecrets),
		"BITBUCKET_EMAIL":     envValue("BITBUCKET_EMAIL", cfg.Bitbucket.Username, includeSecrets),
		"BITBUCKET_API_TOKEN": envValue("BITBUCKET_API_TOKEN", cfg.Bitbucket.Token, includeSecrets),
		"GITHUB_TOKEN":        envValue("GITHUB_TOKEN", cfg.GitHub.Token, includeSecrets),
		"GITLAB_TOKEN":        envValue("GITLAB_TOKEN", cfg.GitLab.Token, includeSecrets),
		"GITLAB_BASE_URL":     envValue("GITLAB_BASE_URL", cfg.GitLab.BaseURL, includeSecrets),
		"TESTRAIL_BASE_URL":   envValue("TESTRAIL_BASE_URL", cfg.TestRail.BaseURL, includeSecrets),
		"TESTRAIL_EMAIL":      envValue("TESTRAIL_EMAIL", cfg.TestRail.Email, includeSecrets),
		"TESTRAIL_API_KEY":    envValue("TESTRAIL_API_KEY", cfg.TestRail.APIKey, includeSecrets),
		"FIGMA_ACCESS_TOKEN":  envValue("FIGMA_ACCESS_TOKEN", cfg.Figma.Token, includeSecrets),
	}
	if cfg.GitHub.APIBase != "" && cfg.GitHub.APIBase != config.DefaultGitHubAPIBase {
		env["GITHUB_API_BASE_URL"] = envValue("GITHUB_API_BASE_URL", cfg.GitHub.APIBase, includeSecrets)
	}
	if cfg.GitHub.WebBase != "" && cfg.GitHub.WebBase != config.DefaultGitHubWebBase {
		env["GITHUB_WEB_BASE_URL"] = envValue("GITHUB_WEB_BASE_URL", cfg.GitHub.WebBase, includeSecrets)
	}
	if cfg.GitHub.APIVersion != "" {
		env["GITHUB_API_VERSION"] = envValue("GITHUB_API_VERSION", cfg.GitHub.APIVersion, includeSecrets)
	}
	if cfg.Figma.APIBase != "" && cfg.Figma.APIBase != config.DefaultFigmaAPIBase {
		env["FIGMA_API_BASE_URL"] = envValue("FIGMA_API_BASE_URL", cfg.Figma.APIBase, includeSecrets)
	}
	return env
}

func envValue(name, value string, include bool) string {
	if include {
		return value
	}
	return "${" + name + "}"
}

func writers(opts Options) (io.Writer, io.Writer) {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	return stdout, stderr
}
