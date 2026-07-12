package config

import "testing"

func TestLoadTreatsUnresolvedEnvReferencesAsEmpty(t *testing.T) {
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
