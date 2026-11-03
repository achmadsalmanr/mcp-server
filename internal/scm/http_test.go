package scm

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveURL(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{name: "relative path", path: "/repos/a/b", want: "https://api.example.com/v4/repos/a/b"},
		{name: "same origin", path: "https://api.example.com/v4/next?page=2", want: "https://api.example.com/v4/next?page=2"},
		{name: "explicit default port", path: "https://api.example.com:443/next", want: "https://api.example.com:443/next"},
		{name: "other host", path: "https://evil.example.net/next", wantErr: true},
		{name: "subdomain", path: "https://x.api.example.com/next", wantErr: true},
		{name: "scheme downgrade", path: "http://api.example.com/next", wantErr: true},
		{name: "other port", path: "https://api.example.com:8443/next", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveURL("https://api.example.com/v4", tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolveURL(%q) = %q, want error", tt.path, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveURL(%q) error = %v", tt.path, err)
			}
			if got != tt.want {
				t.Fatalf("ResolveURL(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestNewHTTPClientStripsCredentialsOnCrossOriginRedirect(t *testing.T) {
	var got http.Header
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
	}))
	defer other.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusFound)
	}))
	defer origin.Close()

	resp, err := NewHTTPClient().Do(credentialedRequest(t, origin.URL))
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	resp.Body.Close()
	for _, name := range credentialHeaders {
		if v := got.Get(name); v != "" {
			t.Errorf("%s forwarded across origins: %q", name, v)
		}
	}
	if got.Get("Accept") != "application/json" {
		t.Errorf("non-credential header Accept was dropped")
	}
}

func TestNewHTTPClientKeepsCredentialsOnSameOriginRedirect(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		got = r.Header.Clone()
	}))
	defer srv.Close()

	resp, err := NewHTTPClient().Do(credentialedRequest(t, srv.URL+"/start"))
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	resp.Body.Close()
	if got.Get("Private-Token") != "glpat-secret" || got.Get("X-Figma-Token") != "figd_secret" {
		t.Fatalf("credentials dropped on same-origin redirect: %v", got)
	}
}

func TestValidPathSegment(t *testing.T) {
	for _, s := range []string{"acme", "my-repo", "my_repo.js", "Repo123"} {
		if !ValidPathSegment(s) {
			t.Errorf("ValidPathSegment(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", ".", "..", "a/b", "a?b", "a#b", "a%2Fb", "a b", "a\\b"} {
		if ValidPathSegment(s) {
			t.Errorf("ValidPathSegment(%q) = true, want false", s)
		}
	}
}

func TestReadBodyRejectsOversizedBody(t *testing.T) {
	if _, err := ReadBody(strings.NewReader(strings.Repeat("x", MaxResponseRead+1))); err == nil {
		t.Fatalf("expected an error for a body over MaxResponseRead")
	}
	b, err := ReadBody(strings.NewReader("ok"))
	if err != nil || string(b) != "ok" {
		t.Fatalf("ReadBody() = %q, %v", b, err)
	}
}

func credentialedRequest(t *testing.T, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer ghp_secret")
	req.Header.Set("PRIVATE-TOKEN", "glpat-secret")
	req.Header.Set("X-Figma-Token", "figd_secret")
	req.Header.Set("Accept", "application/json")
	return req
}
