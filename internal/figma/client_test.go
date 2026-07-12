package figma

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseFileRef(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantKey string
		wantIDs []string
	}{
		{
			name:    "raw key",
			raw:     "AbCdEf123",
			wantKey: "AbCdEf123",
		},
		{
			name:    "file URL",
			raw:     "https://www.figma.com/file/AbCdEf123/Product?type=design",
			wantKey: "AbCdEf123",
		},
		{
			name:    "node URL",
			raw:     "https://www.figma.com/design/AbCdEf123/Product?node-id=12-34",
			wantKey: "AbCdEf123",
			wantIDs: []string{"12:34"},
		},
		{
			name:    "encoded node URL",
			raw:     "https://figma.com/design/AbCdEf123/Product?node-id=12%3A34",
			wantKey: "AbCdEf123",
			wantIDs: []string{"12:34"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFileRef(tt.raw)
			if err != nil {
				t.Fatalf("ParseFileRef() error = %v", err)
			}
			if got.Key != tt.wantKey {
				t.Fatalf("key = %q, want %q", got.Key, tt.wantKey)
			}
			if len(got.NodeIDs) != len(tt.wantIDs) {
				t.Fatalf("node IDs = %#v, want %#v", got.NodeIDs, tt.wantIDs)
			}
			for i := range tt.wantIDs {
				if got.NodeIDs[i] != tt.wantIDs[i] {
					t.Fatalf("node IDs = %#v, want %#v", got.NodeIDs, tt.wantIDs)
				}
			}
		})
	}
}

func TestGetFileBuildsRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/files/AbCdEf123" {
			t.Fatalf("path = %q, want /v1/files/AbCdEf123", r.URL.Path)
		}
		if got := r.Header.Get("X-Figma-Token"); got != "figd_token" {
			t.Fatalf("X-Figma-Token = %q, want figd_token", got)
		}
		q := r.URL.Query()
		if got := q.Get("ids"); got != "1:2,1:3" {
			t.Fatalf("ids = %q, want 1:2,1:3", got)
		}
		if got := q.Get("depth"); got != "2" {
			t.Fatalf("depth = %q, want 2", got)
		}
		if got := q.Get("geometry"); got != "paths" {
			t.Fatalf("geometry = %q, want paths", got)
		}
		if got := q.Get("plugin_data"); got != "shared" {
			t.Fatalf("plugin_data = %q, want shared", got)
		}
		if got := q.Get("branch_data"); got != "true" {
			t.Fatalf("branch_data = %q, want true", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"Design"}`))
	}))
	defer srv.Close()

	client := NewClient("figd_token", srv.URL)
	got, err := client.GetFile(context.Background(), GetFileInput{
		FileKey:    "AbCdEf123",
		IDs:        []string{"1-2", "1:3"},
		Depth:      2,
		Geometry:   "paths",
		PluginData: []string{"shared"},
		BranchData: true,
	})
	if err != nil {
		t.Fatalf("GetFile() error = %v", err)
	}
	if got["name"] != "Design" {
		t.Fatalf("name = %v, want Design", got["name"])
	}
}

func TestGetFileNodesUsesNodeIDFromURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("ids"); got != "12:34" {
			t.Fatalf("ids = %q, want 12:34", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"nodes":{}}`))
	}))
	defer srv.Close()

	client := NewClient("figd_token", srv.URL)
	_, err := client.GetFileNodes(context.Background(), GetFileNodesInput{
		FileKey: "https://www.figma.com/design/AbCdEf123/Product?node-id=12-34",
	})
	if err != nil {
		t.Fatalf("GetFileNodes() error = %v", err)
	}
}

func TestPostCommentBuildsPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["message"] != "Looks good" {
			t.Fatalf("message = %v, want Looks good", payload["message"])
		}
		if payload["comment_id"] != "123" {
			t.Fatalf("comment_id = %v, want 123", payload["comment_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"456"}`))
	}))
	defer srv.Close()

	client := NewClient("figd_token", srv.URL)
	got, err := client.PostComment(context.Background(), PostCommentInput{
		FileKey:   "AbCdEf123",
		Message:   " Looks good ",
		CommentID: "123",
	})
	if err != nil {
		t.Fatalf("PostComment() error = %v", err)
	}
	if got["id"] != "456" {
		t.Fatalf("id = %v, want 456", got["id"])
	}
}
