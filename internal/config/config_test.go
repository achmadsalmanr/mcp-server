package config

import "testing"

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
