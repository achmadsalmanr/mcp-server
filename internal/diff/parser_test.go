package diff

import "testing"

func TestParseAndPageUnifiedDiff(t *testing.T) {
	raw := `diff --git a/app.go b/app.go
--- a/app.go
+++ b/app.go
@@ -1,3 +1,4 @@
 package main
-var old = true
+var next = true
+var added = true
 func main() {}
`
	parsed, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(parsed.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(parsed.Files))
	}
	line, err := parsed.FindLine("app.go", 2)
	if err != nil {
		t.Fatalf("FindLine() error = %v", err)
	}
	if line.Type != LineAdded || line.Content != "var next = true" {
		t.Fatalf("line = %#v, want added next line", line)
	}
	page, err := parsed.GetPage(PageInput{PRURL: "https://example/pr/1", MaxLines: 2})
	if err != nil {
		t.Fatalf("GetPage() error = %v", err)
	}
	if !page.Truncated || page.NextCursor == "" {
		t.Fatalf("page truncation = (%v, %q), want truncated with cursor", page.Truncated, page.NextCursor)
	}
	next, err := parsed.GetPage(PageInput{PRURL: "https://example/pr/1", MaxLines: 2, Cursor: page.NextCursor})
	if err != nil {
		t.Fatalf("GetPage(next) error = %v", err)
	}
	if len(next.Files) == 0 {
		t.Fatalf("next page returned no files")
	}
}
