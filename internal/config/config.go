package config

import (
	"os"
	"strings"
)

const (
	DefaultGitHubAPIBase = "https://api.github.com"
	DefaultGitHubWebBase = "https://github.com"
	DefaultGitLabBase    = "https://gitlab.com"
)

type Config struct {
	Bitbucket BitbucketConfig
	GitHub    GitHubConfig
	GitLab    GitLabConfig
	TestRail  TestRailConfig
}

type BitbucketConfig struct {
	Workspace string
	Username  string
	Token     string
	APIBase   string
}

type GitHubConfig struct {
	Token      string
	APIBase    string
	WebBase    string
	APIVersion string
}

type GitLabConfig struct {
	Token   string
	BaseURL string
	APIBase string
}

type TestRailConfig struct {
	BaseURL string
	Email   string
	APIKey  string
}

type ProviderStatus struct {
	Name       string   `json:"name"`
	Configured bool     `json:"configured"`
	Missing    []string `json:"missing,omitempty"`
	Notes      []string `json:"notes,omitempty"`
}

func Load() (Config, []string) {
	var warnings []string
	cfg := Config{
		Bitbucket: BitbucketConfig{
			Workspace: env("BITBUCKET_WORKSPACE"),
			Username:  env("BITBUCKET_EMAIL"),
			Token:     env("BITBUCKET_API_TOKEN"),
			APIBase:   envDefault("BITBUCKET_API_BASE_URL", "https://api.bitbucket.org/2.0"),
		},
		GitHub: GitHubConfig{
			Token:      env("GITHUB_TOKEN"),
			APIBase:    envDefault("GITHUB_API_BASE_URL", DefaultGitHubAPIBase),
			WebBase:    envDefault("GITHUB_WEB_BASE_URL", DefaultGitHubWebBase),
			APIVersion: env("GITHUB_API_VERSION"),
		},
		GitLab: GitLabConfig{
			Token:   env("GITLAB_TOKEN"),
			BaseURL: envDefault("GITLAB_BASE_URL", DefaultGitLabBase),
		},
		TestRail: TestRailConfig{
			BaseURL: strings.TrimRight(env("TESTRAIL_BASE_URL"), "/"),
			Email:   env("TESTRAIL_EMAIL"),
			APIKey:  env("TESTRAIL_API_KEY"),
		},
	}

	if cfg.Bitbucket.Username == "" {
		if value := env("BITBUCKET_USERNAME"); value != "" {
			cfg.Bitbucket.Username = value
			warnings = append(warnings, "BITBUCKET_USERNAME is deprecated; use BITBUCKET_EMAIL")
		}
	}
	if cfg.Bitbucket.Token == "" {
		if value := env("BITBUCKET_APP_PASSWORD"); value != "" {
			cfg.Bitbucket.Token = value
			warnings = append(warnings, "BITBUCKET_APP_PASSWORD is deprecated; use BITBUCKET_API_TOKEN")
		}
	}
	if cfg.TestRail.APIKey == "" {
		if value := env("TESTRAIL_TOKEN"); value != "" {
			cfg.TestRail.APIKey = value
			warnings = append(warnings, "TESTRAIL_TOKEN is deprecated; use TESTRAIL_API_KEY")
		}
	}
	if cfg.TestRail.Email == "" {
		if value := env("TESTRAIL_USERNAME"); value != "" {
			cfg.TestRail.Email = value
			warnings = append(warnings, "TESTRAIL_USERNAME is deprecated; use TESTRAIL_EMAIL")
		}
	}

	cfg.GitLab.APIBase = envDefault("GITLAB_API_BASE_URL", strings.TrimRight(cfg.GitLab.BaseURL, "/")+"/api/v4")
	return cfg, warnings
}

func (c Config) Statuses() []ProviderStatus {
	return []ProviderStatus{
		{
			Name:       "bitbucket",
			Configured: c.Bitbucket.Configured(),
			Missing:    missing(c.Bitbucket.Workspace, "BITBUCKET_WORKSPACE", c.Bitbucket.Username, "BITBUCKET_EMAIL", c.Bitbucket.Token, "BITBUCKET_API_TOKEN"),
		},
		{
			Name:       "github",
			Configured: true,
			Notes:      optionalTokenNote(c.GitHub.Token, "GITHUB_TOKEN", "public repositories can be read without a token; private repositories and writes need one"),
		},
		{
			Name:       "gitlab",
			Configured: true,
			Notes:      optionalTokenNote(c.GitLab.Token, "GITLAB_TOKEN", "public projects can be read without a token; private projects and writes need one"),
		},
		{
			Name:       "testrail",
			Configured: c.TestRail.Configured(),
			Missing:    missing(c.TestRail.BaseURL, "TESTRAIL_BASE_URL", c.TestRail.Email, "TESTRAIL_EMAIL", c.TestRail.APIKey, "TESTRAIL_API_KEY"),
		},
	}
}

func (c BitbucketConfig) Configured() bool {
	return c.Workspace != "" && c.Username != "" && c.Token != ""
}

func (c TestRailConfig) Configured() bool {
	return c.BaseURL != "" && c.Email != "" && c.APIKey != ""
}

func envDefault(name, fallback string) string {
	if value := env(name); value != "" {
		return strings.TrimRight(value, "/")
	}
	return strings.TrimRight(fallback, "/")
}

func env(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if isEnvReference(value) {
		return ""
	}
	return value
}

func isEnvReference(value string) bool {
	return strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}")
}

func missing(valuesAndNames ...string) []string {
	var out []string
	for i := 0; i+1 < len(valuesAndNames); i += 2 {
		if valuesAndNames[i] == "" {
			out = append(out, valuesAndNames[i+1])
		}
	}
	return out
}

func optionalTokenNote(token, envName, note string) []string {
	if token != "" {
		return nil
	}
	return []string{envName + " is not set: " + note}
}
