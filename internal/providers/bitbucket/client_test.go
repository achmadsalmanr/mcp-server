package bitbucket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseURL(t *testing.T) {
	ref, err := ParseURL("https://bitbucket.org/acme/widgets/pull-requests/42")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	if ref.Workspace != "acme" || ref.RepoSlug != "widgets" || ref.ID != 42 {
		t.Fatalf("ref = %#v", ref)
	}
}

func TestParseURLRejectsPathInjection(t *testing.T) {
	for _, raw := range []string{
		"https://bitbucket.org/acme/../pull-requests/1",
		"https://bitbucket.org/acme/%2E%2E/pull-requests/1",
		"https://bitbucket.org/acme/widgets%3Fx=1/pull-requests/1",
	} {
		if _, err := ParseURL(raw); err == nil {
			t.Errorf("ParseURL(%q) succeeded, want error", raw)
		}
	}
}

func TestPaginationDoesNotSendTokenToOtherHost(t *testing.T) {
	var leaked string
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("Authorization")
		w.Write([]byte(`{"values":[]}`))
	}))
	defer attacker.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"values":[],"next":"` + attacker.URL + `/steal"}`))
	}))
	defer api.Close()

	client := NewClient("acme", "me@example.com", "secret", api.URL)
	_, err := client.ListPullRequestCommits(context.Background(), "https://bitbucket.org/acme/widgets/pull-requests/1")
	if leaked != "" {
		t.Fatalf("Authorization leaked to another host: %q", leaked)
	}
	if err == nil {
		t.Fatalf("expected an error for a next link on another host")
	}
}
