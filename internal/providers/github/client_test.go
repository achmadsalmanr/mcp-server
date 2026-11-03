package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSupportsURL(t *testing.T) {
	client := NewClient("", "https://api.github.com", "https://github.com", "")
	if !client.SupportsURL("https://github.com/acme/widgets/pull/42") {
		t.Fatalf("expected GitHub URL to be supported")
	}
	if client.SupportsURL("https://gitlab.com/acme/widgets/-/merge_requests/42") {
		t.Fatalf("expected GitLab URL not to be supported")
	}
}

func TestSupportsURLRejectsPathInjection(t *testing.T) {
	client := NewClient("", "https://api.github.com", "https://github.com", "")
	for _, raw := range []string{
		"https://github.com/acme/../pull/1",
		"https://github.com/%2E%2E/widgets/pull/1",
		"https://github.com/acme/widgets%3Fx=1/pull/1",
		"https://github.com/acme/wid%23gets/pull/1",
	} {
		if client.SupportsURL(raw) {
			t.Errorf("SupportsURL(%q) = true, want false", raw)
		}
	}
}

func TestPaginationDoesNotSendTokenToOtherHost(t *testing.T) {
	var leaked string
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("Authorization")
		w.Write([]byte("[]"))
	}))
	defer attacker.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", "<"+attacker.URL+"/steal>; rel=\"next\"")
		w.Write([]byte("[]"))
	}))
	defer api.Close()

	client := NewClient("ghp_secret", api.URL, "https://github.com", "")
	_, err := client.ListPullRequestCommits(context.Background(), "https://github.com/acme/widgets/pull/1")
	if leaked != "" {
		t.Fatalf("Authorization leaked to another host: %q", leaked)
	}
	if err == nil {
		t.Fatalf("expected an error for a next link on another host")
	}
}
