package config

import "testing"

func TestLoadTreatsUnresolvedEnvReferencesAsEmpty(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "${GITHUB_TOKEN}")
	t.Setenv("GITLAB_BASE_URL", "${GITLAB_BASE_URL}")
	t.Setenv("TESTRAIL_API_KEY", "${TESTRAIL_API_KEY}")

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
}
