package scm

import "strings"

const MaxBodyRead = 64 * 1024

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
