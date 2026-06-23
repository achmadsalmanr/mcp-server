package bitbucket

import "testing"

func TestParseURL(t *testing.T) {
	ref, err := ParseURL("https://bitbucket.org/acme/widgets/pull-requests/42")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	if ref.Workspace != "acme" || ref.RepoSlug != "widgets" || ref.ID != 42 {
		t.Fatalf("ref = %#v", ref)
	}
}
