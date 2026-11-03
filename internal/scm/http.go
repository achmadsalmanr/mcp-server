package scm

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const MaxBodyRead = 64 * 1024

// MaxResponseRead caps successful response bodies so a huge diff or file
// cannot exhaust memory. It matches the stdio request line limit.
const MaxResponseRead = 64 * 1024 * 1024

const maxRedirects = 10

// credentialHeaders are dropped when a redirect leaves the original origin.
// net/http only strips Authorization and Cookie, not provider-specific headers.
var credentialHeaders = []string{"Authorization", "Private-Token", "X-Figma-Token", "Cookie"}

type APIError struct {
	Provider   string
	StatusCode int
	Path       string
	Body       string
}

func (e *APIError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		body = "<empty body>"
	}
	return e.Provider + " API error " + itoa(e.StatusCode) + " on " + e.Path + ": " + body
}

// NewHTTPClient returns a client that never forwards credentials across origins.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if !sameOrigin(via[0].URL, req.URL) {
				for _, name := range credentialHeaders {
					req.Header.Del(name)
				}
			}
			return nil
		},
	}
}

// ResolveURL joins a relative path onto apiBase. Absolute URLs, such as
// pagination links returned by the API, must share apiBase's origin so the
// caller never attaches credentials to a request for another host.
func ResolveURL(apiBase, path string) (string, error) {
	target, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("invalid request URL %q: %w", path, err)
	}
	if !target.IsAbs() {
		return apiBase + path, nil
	}
	base, err := url.Parse(apiBase)
	if err != nil {
		return "", fmt.Errorf("invalid API base URL %q: %w", apiBase, err)
	}
	if !sameOrigin(base, target) {
		return "", fmt.Errorf("refusing to send credentials to %s://%s; expected API origin %s://%s", target.Scheme, target.Host, base.Scheme, base.Host)
	}
	return path, nil
}

// ReadBody reads a successful response body up to MaxResponseRead.
func ReadBody(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxResponseRead+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxResponseRead {
		return nil, errors.New("response body exceeds " + itoa(MaxResponseRead) + " bytes")
	}
	return b, nil
}

// LimitBody bounds a response body that is streamed into a JSON decoder.
func LimitBody(r io.Reader) io.Reader {
	return io.LimitReader(r, MaxResponseRead)
}

// ValidPathSegment reports whether s is safe to place in an API URL path
// unescaped: no traversal, separators, query, or fragment characters.
func ValidPathSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) &&
		strings.EqualFold(a.Hostname(), b.Hostname()) &&
		portOrDefault(a) == portOrDefault(b)
}

func portOrDefault(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return "443"
	case "http":
		return "80"
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
