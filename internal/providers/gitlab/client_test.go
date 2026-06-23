package gitlab

import "testing"

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
