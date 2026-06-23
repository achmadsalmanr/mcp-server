package github

import "testing"

func TestSupportsURL(t *testing.T) {
	client := NewClient("", "https://api.github.com", "https://github.com", "")
	if !client.SupportsURL("https://github.com/acme/widgets/pull/42") {
		t.Fatalf("expected GitHub URL to be supported")
	}
	if client.SupportsURL("https://gitlab.com/acme/widgets/-/merge_requests/42") {
		t.Fatalf("expected GitLab URL not to be supported")
	}
}
