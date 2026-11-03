package config

import (
	"strings"
	"testing"
)

func TestLoadTreatsUnresolvedEnvReferencesAsEmpty(t *testing.T) {
	withPersistentEnvLookup(t, nil)
	t.Setenv("GITHUB_TOKEN", "${GITHUB_TOKEN}")
	t.Setenv("GITLAB_BASE_URL", "${GITLAB_BASE_URL}")
	t.Setenv("TESTRAIL_API_KEY", "${TESTRAIL_API_KEY}")
	t.Setenv("FIGMA_ACCESS_TOKEN", "${FIGMA_ACCESS_TOKEN}")
	t.Setenv("FIGMA_API_BASE_URL", "${FIGMA_API_BASE_URL}")

	cfg, _ := Load()
	if cfg.GitHub.Token != "" {
		t.Fatalf("GitHub token = %q, want empty unresolved reference", cfg.GitHub.Token)
	}
	if cfg.GitLab.BaseURL != DefaultGitLabBase {
		t.Fatalf("GitLab base URL = %q, want default %q", cfg.GitLab.BaseURL, DefaultGitLabBase)
	}
	if cfg.TestRail.APIKey != "" {
		t.Fatalf("TestRail API key = %q, want empty unresolved reference", cfg.TestRail.APIKey)
	}
	if cfg.Figma.Token != "" {
		t.Fatalf("Figma token = %q, want empty unresolved reference", cfg.Figma.Token)
	}
	if cfg.Figma.APIBase != DefaultFigmaAPIBase {
		t.Fatalf("Figma API base URL = %q, want default %q", cfg.Figma.APIBase, DefaultFigmaAPIBase)
	}
}

func TestLoadReadsPersistentEnvironmentWhenProcessEnvironmentIsMissing(t *testing.T) {
	withPersistentEnvLookup(t, mapPersistentEnv(map[string]string{
		"GITHUB_TOKEN":       "github-token",
		"FIGMA_ACCESS_TOKEN": "figma-token",
	}))
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("FIGMA_ACCESS_TOKEN", "")

	cfg, _ := Load()
	if cfg.GitHub.Token != "github-token" {
		t.Fatalf("GitHub token = %q, want persistent environment value", cfg.GitHub.Token)
	}
	if cfg.Figma.Token != "figma-token" {
		t.Fatalf("Figma token = %q, want persistent environment value", cfg.Figma.Token)
	}
}

func TestLoadResolvesLiteralEnvReferencesFromPersistentEnvironment(t *testing.T) {
	withPersistentEnvLookup(t, mapPersistentEnv(map[string]string{
		"GITHUB_TOKEN":       "github-token",
		"FIGMA_ACCESS_TOKEN": "figma-token",
	}))
	t.Setenv("GITHUB_TOKEN", "${GITHUB_TOKEN}")
	t.Setenv("FIGMA_ACCESS_TOKEN", "${FIGMA_ACCESS_TOKEN}")

	cfg, _ := Load()
	if cfg.GitHub.Token != "github-token" {
		t.Fatalf("GitHub token = %q, want persistent environment value", cfg.GitHub.Token)
	}
	if cfg.Figma.Token != "figma-token" {
		t.Fatalf("Figma token = %q, want persistent environment value", cfg.Figma.Token)
	}
}

func TestLoadResolvesLiteralEnvReferencesToDifferentVariableNames(t *testing.T) {
	withPersistentEnvLookup(t, mapPersistentEnv(map[string]string{
		"STRATIVY_GITHUB_TOKEN": "github-token",
	}))
	t.Setenv("GITHUB_TOKEN", "${STRATIVY_GITHUB_TOKEN}")

	cfg, _ := Load()
	if cfg.GitHub.Token != "github-token" {
		t.Fatalf("GitHub token = %q, want referenced persistent environment value", cfg.GitHub.Token)
	}
}

func TestLoadWarnsAboutInsecureBaseURLs(t *testing.T) {
	withPersistentEnvLookup(t, nil)
	clearBaseURLEnv(t)
	t.Setenv("GITLAB_BASE_URL", "http://gitlab.internal")
	t.Setenv("TESTRAIL_BASE_URL", "http://localhost:8080")

	_, warnings := Load()
	if !containsWarning(warnings, "GITLAB_BASE_URL/GITLAB_API_BASE_URL does not use https") {
		t.Fatalf("warnings = %q, want plain-HTTP GitLab warning", warnings)
	}
	if containsWarning(warnings, "TESTRAIL_BASE_URL") {
		t.Fatalf("warnings = %q, want no warning for loopback HTTP", warnings)
	}
}

func TestLoadWarnsAboutPartialGitHubEnterpriseConfig(t *testing.T) {
	withPersistentEnvLookup(t, nil)
	clearBaseURLEnv(t)

	if _, warnings := Load(); containsWarning(warnings, "GITHUB_WEB_BASE_URL") {
		t.Fatalf("warnings = %q, want none for default GitHub config", warnings)
	}
	t.Setenv("GITHUB_WEB_BASE_URL", "https://ghe.example.com")
	if _, warnings := Load(); !containsWarning(warnings, "set GITHUB_WEB_BASE_URL and GITHUB_API_BASE_URL together") {
		t.Fatalf("warnings = %q, want partial GitHub Enterprise warning", warnings)
	}
	t.Setenv("GITHUB_API_BASE_URL", "https://ghe.example.com/api/v3")
	if _, warnings := Load(); containsWarning(warnings, "GITHUB_WEB_BASE_URL") {
		t.Fatalf("warnings = %q, want none for complete GitHub Enterprise config", warnings)
	}
}

func clearBaseURLEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"BITBUCKET_API_BASE_URL", "GITHUB_API_BASE_URL", "GITHUB_WEB_BASE_URL",
		"GITLAB_BASE_URL", "GITLAB_API_BASE_URL", "TESTRAIL_BASE_URL", "FIGMA_API_BASE_URL",
	} {
		t.Setenv(name, "")
	}
}

func containsWarning(warnings []string, substr string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, substr) {
			return true
		}
	}
	return false
}

func withPersistentEnvLookup(t *testing.T, lookup func(string) (string, bool)) {
	t.Helper()
	previous := lookupPersistentEnvironment
	lookupPersistentEnvironment = lookup
	t.Cleanup(func() {
		lookupPersistentEnvironment = previous
	})
}

func mapPersistentEnv(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}
