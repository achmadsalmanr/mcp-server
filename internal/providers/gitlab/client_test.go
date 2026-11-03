package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSupportsURL(t *testing.T) {
	client := NewClient("", "https://gitlab.com", "https://gitlab.com/api/v4")
	if !client.SupportsURL("https://gitlab.com/group/subgroup/project/-/merge_requests/42") {
		t.Fatalf("expected GitLab URL to be supported")
	}
	if !client.SupportsURL("https://gitlab.com/group/project/merge_requests/42") {
		t.Fatalf("expected legacy GitLab MR URL to be supported")
	}
	if client.SupportsURL("https://github.com/acme/widgets/pull/42") {
		t.Fatalf("expected GitHub URL not to be supported")
	}
}

func TestSupportsURLRejectsPathTraversal(t *testing.T) {
	client := NewClient("", "https://gitlab.com", "https://gitlab.com/api/v4")
	for _, raw := range []string{
		"https://gitlab.com/../-/merge_requests/1",
		"https://gitlab.com/group/%2E%2E/-/merge_requests/1",
		"https://gitlab.com/group/pro%3Fject/-/merge_requests/1",
	} {
		if client.SupportsURL(raw) {
			t.Errorf("SupportsURL(%q) = true, want false", raw)
		}
	}
}

func TestPaginationDoesNotSendTokenToOtherHost(t *testing.T) {
	var leaked string
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("PRIVATE-TOKEN")
		w.Write([]byte("[]"))
	}))
	defer attacker.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", "<"+attacker.URL+"/steal>; rel=\"next\"")
		w.Write([]byte("[]"))
	}))
	defer api.Close()

	client := NewClient("glpat-secret", "https://gitlab.com", api.URL)
	_, err := client.ListPullRequestCommits(context.Background(), "https://gitlab.com/group/project/-/merge_requests/1")
	if leaked != "" {
		t.Fatalf("PRIVATE-TOKEN leaked to another host: %q", leaked)
	}
	if err == nil {
		t.Fatalf("expected an error for a next link on another host")
	}
}

func TestRedirectDoesNotSendTokenToOtherHost(t *testing.T) {
	var leaked string
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("PRIVATE-TOKEN")
		w.Write([]byte("[]"))
	}))
	defer attacker.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, attacker.URL+"/steal", http.StatusFound)
	}))
	defer api.Close()

	client := NewClient("glpat-secret", "https://gitlab.com", api.URL)
	client.ListPullRequestCommits(context.Background(), "https://gitlab.com/group/project/-/merge_requests/1")
	if leaked != "" {
		t.Fatalf("PRIVATE-TOKEN leaked through redirect: %q", leaked)
	}
}
