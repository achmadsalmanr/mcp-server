package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/achmadsalmanr/mcp-server/internal/diff"
	"github.com/achmadsalmanr/mcp-server/internal/scm"
)

const maxPaginationPages = 100

type Client struct {
	token   string
	baseURL string
	apiBase string
	webHost string
	http    *http.Client
}

type MergeRequestRef struct {
	ProjectPath string
	IID         int
}

type mergeRequestRaw struct {
	ID          int    `json:"id"`
	IID         int    `json:"iid"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	WebURL      string `json:"web_url"`
	Source      string `json:"source_branch"`
	Target      string `json:"target_branch"`
	SHA         string `json:"sha"`
	CreatedAt   string `json:"created_at"`
	Author      struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"author"`
	DiffRefs struct {
		BaseSHA  string `json:"base_sha"`
		StartSHA string `json:"start_sha"`
		HeadSHA  string `json:"head_sha"`
	} `json:"diff_refs"`
}

func NewClient(token, baseURL, apiBase string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	webHost := "gitlab.com"
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		webHost = u.Host
	}
	return &Client{
		token:   token,
		baseURL: baseURL,
		apiBase: strings.TrimRight(apiBase, "/"),
		webHost: webHost,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Provider() string { return scm.ProviderGitLab }

func (c *Client) SupportsURL(rawURL string) bool {
	_, err := c.parseURL(rawURL)
	return err == nil
}

func (c *Client) parseURL(rawURL string) (MergeRequestRef, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return MergeRequestRef{}, fmt.Errorf("invalid URL: %w", err)
	}
	if !strings.EqualFold(u.Host, c.webHost) {
		return MergeRequestRef{}, fmt.Errorf("URL host must be %s, got %q", c.webHost, u.Host)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(parts); i++ {
		if i+1 < len(parts) && parts[i] == "-" && parts[i+1] == "merge_requests" {
			if i+2 >= len(parts) {
				break
			}
			iid, err := strconv.Atoi(parts[i+2])
			if err != nil {
				return MergeRequestRef{}, fmt.Errorf("merge request IID %q is not an integer", parts[i+2])
			}
			project := strings.Join(parts[:i], "/")
			if project == "" {
				return MergeRequestRef{}, fmt.Errorf("project path must not be empty")
			}
			return MergeRequestRef{ProjectPath: project, IID: iid}, nil
		}
		if parts[i] == "merge_requests" {
			if i+1 >= len(parts) {
				break
			}
			iid, err := strconv.Atoi(parts[i+1])
			if err != nil {
				return MergeRequestRef{}, fmt.Errorf("merge request IID %q is not an integer", parts[i+1])
			}
			project := strings.Join(parts[:i], "/")
			if project == "" {
				return MergeRequestRef{}, fmt.Errorf("project path must not be empty")
			}
			return MergeRequestRef{ProjectPath: project, IID: iid}, nil
		}
	}
	return MergeRequestRef{}, fmt.Errorf("URL does not match %s/{group}/{project}/-/merge_requests/{iid}: %q", c.baseURL, rawURL)
}

func (c *Client) GetPullRequest(ctx context.Context, prURL string) (*scm.PullRequest, error) {
	ref, raw, err := c.getMergeRequest(ctx, prURL)
	if err != nil {
		return nil, err
	}
	author := raw.Author.Name
	if author == "" {
		author = raw.Author.Username
	}
	head := raw.DiffRefs.HeadSHA
	if head == "" {
		head = raw.SHA
	}
	return &scm.PullRequest{
		Provider:         c.Provider(),
		Repository:       ref.ProjectPath,
		ID:               raw.IID,
		Title:            raw.Title,
		Description:      raw.Description,
		Author:           author,
		SourceBranch:     raw.Source,
		DestBranch:       raw.Target,
		State:            raw.State,
		CreatedAt:        raw.CreatedAt,
		SourceCommitHash: head,
		WebURL:           raw.WebURL,
		APIURL:           c.mrBase(ref),
	}, nil
}

func (c *Client) GetPullRequestDiff(ctx context.Context, prURL string) (string, error) {
	ref, err := c.parseURL(prURL)
	if err != nil {
		return "", err
	}
	type rawDiff struct {
		OldPath     string `json:"old_path"`
		NewPath     string `json:"new_path"`
		Diff        string `json:"diff"`
		NewFile     bool   `json:"new_file"`
		RenamedFile bool   `json:"renamed_file"`
		DeletedFile bool   `json:"deleted_file"`
	}
	items, err := getAllPages[rawDiff](ctx, c, c.mrBase(ref)+"/diffs?per_page=100")
	if err != nil {
		return "", fmt.Errorf("fetching merge request diffs: %w", err)
	}
	var b strings.Builder
	for _, item := range items {
		oldPath := item.OldPath
		newPath := item.NewPath
		if oldPath == "" {
			oldPath = newPath
		}
		if newPath == "" {
			newPath = oldPath
		}
		b.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", oldPath, newPath))
		if item.NewFile {
			b.WriteString("new file mode 100644\n")
		}
		if item.DeletedFile {
			b.WriteString("deleted file mode 100644\n")
		}
		if item.RenamedFile {
			b.WriteString("rename from " + oldPath + "\n")
			b.WriteString("rename to " + newPath + "\n")
		}
		if item.DeletedFile {
			b.WriteString("--- a/" + oldPath + "\n")
			b.WriteString("+++ /dev/null\n")
		} else if item.NewFile {
			b.WriteString("--- /dev/null\n")
			b.WriteString("+++ b/" + newPath + "\n")
		} else {
			b.WriteString("--- a/" + oldPath + "\n")
			b.WriteString("+++ b/" + newPath + "\n")
		}
		b.WriteString(item.Diff)
		if !strings.HasSuffix(item.Diff, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String(), nil
}

func (c *Client) ListPullRequestCommits(ctx context.Context, prURL string) ([]scm.Commit, error) {
	ref, err := c.parseURL(prURL)
	if err != nil {
		return nil, err
	}
	type raw struct {
		ID        string `json:"id"`
		ShortID   string `json:"short_id"`
		Title     string `json:"title"`
		Message   string `json:"message"`
		Author    string `json:"author_name"`
		WebURL    string `json:"web_url"`
		CreatedAt string `json:"created_at"`
	}
	items, err := getAllPages[raw](ctx, c, c.mrBase(ref)+"/commits?per_page=100")
	if err != nil {
		return nil, fmt.Errorf("fetching commits: %w", err)
	}
	commits := make([]scm.Commit, 0, len(items))
	for _, item := range items {
		hash := item.ShortID
		if hash == "" {
			hash = item.ID
			if len(hash) > 12 {
				hash = hash[:12]
			}
		}
		message := item.Title
		if message == "" {
			message = strings.SplitN(item.Message, "\n", 2)[0]
		}
		commits = append(commits, scm.Commit{Hash: hash, Message: message, Author: item.Author, WebURL: item.WebURL})
	}
	return commits, nil
}

func (c *Client) ListPullRequestComments(ctx context.Context, prURL string) ([]scm.Comment, error) {
	ref, err := c.parseURL(prURL)
	if err != nil {
		return nil, err
	}
	discussions, err := getAllPages[discussionRaw](ctx, c, c.mrBase(ref)+"/discussions?per_page=100")
	if err != nil {
		return nil, fmt.Errorf("fetching discussions: %w", err)
	}
	comments := make([]scm.Comment, 0)
	for _, discussion := range discussions {
		for _, note := range discussion.Notes {
			if note.System {
				continue
			}
			author := note.Author.Name
			if author == "" {
				author = note.Author.Username
			}
			line := 0
			file := ""
			if note.Position.NewLine != nil {
				line = *note.Position.NewLine
			}
			if note.Position.NewPath != "" {
				file = note.Position.NewPath
			}
			comments = append(comments, scm.Comment{
				ID:        discussion.ID + ":" + strconv.Itoa(note.ID),
				ThreadID:  discussion.ID,
				Body:      note.Body,
				File:      file,
				Line:      line,
				Author:    author,
				CreatedAt: note.CreatedAt,
				Resolved:  note.Resolved,
			})
		}
	}
	return comments, nil
}

func (c *Client) CreatePullRequestComment(ctx context.Context, prURL string, input scm.CreateCommentInput) (*scm.Comment, error) {
	ref, rawMR, err := c.getMergeRequest(ctx, prURL)
	if err != nil {
		return nil, err
	}
	if input.FilePath == "" {
		var out noteRaw
		if err := c.doJSON(ctx, http.MethodPost, c.mrBase(ref)+"/notes", map[string]string{"body": input.Body}, &out); err != nil {
			return nil, fmt.Errorf("creating note: %w", err)
		}
		return noteToComment("", out), nil
	}
	oldPath := input.FilePath
	if rawDiff, err := c.GetPullRequestDiff(ctx, prURL); err == nil {
		if parsed, err := diff.Parse(rawDiff); err == nil {
			oldPath = parsed.OldPathFor(input.FilePath)
		}
	}
	baseSHA := rawMR.DiffRefs.BaseSHA
	startSHA := rawMR.DiffRefs.StartSHA
	headSHA := rawMR.DiffRefs.HeadSHA
	if startSHA == "" {
		startSHA = baseSHA
	}
	if headSHA == "" {
		headSHA = rawMR.SHA
	}
	payload := map[string]any{
		"body": input.Body,
		"position": map[string]any{
			"position_type": "text",
			"base_sha":      baseSHA,
			"start_sha":     startSHA,
			"head_sha":      headSHA,
			"old_path":      oldPath,
			"new_path":      input.FilePath,
			"new_line":      input.NewLine,
		},
	}
	var discussion discussionRaw
	if err := c.doJSON(ctx, http.MethodPost, c.mrBase(ref)+"/discussions", payload, &discussion); err != nil {
		return nil, fmt.Errorf("creating discussion: %w", err)
	}
	if len(discussion.Notes) == 0 {
		return &scm.Comment{ID: discussion.ID, ThreadID: discussion.ID, Body: input.Body, File: input.FilePath, Line: input.NewLine}, nil
	}
	return noteToComment(discussion.ID, discussion.Notes[0]), nil
}

func (c *Client) ListPullRequestTasks(context.Context, string) ([]scm.Task, error) {
	return nil, scm.ErrUnsupported
}

func (c *Client) CreatePullRequestTask(context.Context, string, scm.CreateTaskInput) (*scm.Task, error) {
	return nil, scm.ErrUnsupported
}

func (c *Client) UpdatePullRequestTask(context.Context, string, string, scm.UpdateTaskInput) (*scm.Task, error) {
	return nil, scm.ErrUnsupported
}

func (c *Client) ListPullRequestStatuses(ctx context.Context, prURL string) ([]scm.Status, error) {
	ref, rawMR, err := c.getMergeRequest(ctx, prURL)
	if err != nil {
		return nil, err
	}
	sha := rawMR.DiffRefs.HeadSHA
	if sha == "" {
		sha = rawMR.SHA
	}
	type raw struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		TargetURL  string `json:"target_url"`
		CreatedAt  string `json:"created_at"`
		StartedAt  string `json:"started_at"`
		FinishedAt string `json:"finished_at"`
	}
	path := fmt.Sprintf("/projects/%s/repository/commits/%s/statuses?per_page=100", url.PathEscape(ref.ProjectPath), url.PathEscape(sha))
	items, err := getAllPages[raw](ctx, c, path)
	if err != nil {
		return nil, fmt.Errorf("fetching statuses: %w", err)
	}
	statuses := make([]scm.Status, 0, len(items))
	for _, item := range items {
		updated := item.FinishedAt
		if updated == "" {
			updated = item.StartedAt
		}
		statuses = append(statuses, scm.Status{
			Key:       item.Name,
			Name:      item.Name,
			State:     item.Status,
			URL:       item.TargetURL,
			CreatedAt: item.CreatedAt,
			UpdatedAt: updated,
		})
	}
	return statuses, nil
}

func (c *Client) ApprovePullRequest(ctx context.Context, prURL string) error {
	ref, err := c.parseURL(prURL)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodPost, c.mrBase(ref)+"/approve", nil, nil)
}

func (c *Client) RequestChanges(context.Context, string) error {
	return scm.ErrUnsupported
}

func (c *Client) ResolveCommentThread(ctx context.Context, prURL string, commentID string) error {
	return c.setDiscussionResolved(ctx, prURL, commentID, true)
}

func (c *Client) ReopenCommentThread(ctx context.Context, prURL string, commentID string) error {
	return c.setDiscussionResolved(ctx, prURL, commentID, false)
}

func (c *Client) getMergeRequest(ctx context.Context, prURL string) (MergeRequestRef, mergeRequestRaw, error) {
	ref, err := c.parseURL(prURL)
	if err != nil {
		return MergeRequestRef{}, mergeRequestRaw{}, err
	}
	var raw mergeRequestRaw
	if err := c.doJSON(ctx, http.MethodGet, c.mrBase(ref), nil, &raw); err != nil {
		return MergeRequestRef{}, mergeRequestRaw{}, fmt.Errorf("fetching merge request: %w", err)
	}
	return ref, raw, nil
}

func (c *Client) mrBase(ref MergeRequestRef) string {
	return fmt.Sprintf("/projects/%s/merge_requests/%d", url.PathEscape(ref.ProjectPath), ref.IID)
}

func (c *Client) setDiscussionResolved(ctx context.Context, prURL string, commentID string, resolved bool) error {
	ref, err := c.parseURL(prURL)
	if err != nil {
		return err
	}
	discussionID := strings.Split(commentID, ":")[0]
	if discussionID == "" {
		return fmt.Errorf("comment_id must include a GitLab discussion ID")
	}
	path := fmt.Sprintf("%s/discussions/%s?resolved=%t", c.mrBase(ref), url.PathEscape(discussionID), resolved)
	return c.doJSON(ctx, http.MethodPut, path, nil, nil)
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, scm.MaxBodyRead))
		return &scm.APIError{Provider: c.Provider(), StatusCode: resp.StatusCode, Path: path, Body: string(b)}
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decoding response from %s: %w", path, err)
		}
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	reqURL := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		reqURL = c.apiBase + path
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func getAllPages[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var result []T
	next := path
	seen := map[string]bool{}
	pages := 0
	for next != "" {
		if pages >= maxPaginationPages {
			return result, fmt.Errorf("pagination exceeded %d pages", maxPaginationPages)
		}
		if seen[next] {
			return result, fmt.Errorf("pagination detected repeated next URL: %q", next)
		}
		seen[next] = true
		pages++
		var page []T
		req, err := c.newRequest(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP GET %s: %w", next, err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, scm.MaxBodyRead))
			resp.Body.Close()
			return nil, &scm.APIError{Provider: c.Provider(), StatusCode: resp.StatusCode, Path: next, Body: string(b)}
		}
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decoding page from %s: %w", next, err)
		}
		next = parseNextLink(resp.Header.Get("Link"))
		resp.Body.Close()
		result = append(result, page...)
	}
	return result, nil
}

func parseNextLink(link string) string {
	for _, part := range strings.Split(link, ",") {
		sections := strings.Split(part, ";")
		if len(sections) < 2 {
			continue
		}
		if strings.TrimSpace(sections[1]) != `rel="next"` {
			continue
		}
		target := strings.TrimSpace(sections[0])
		target = strings.TrimPrefix(target, "<")
		target = strings.TrimSuffix(target, ">")
		return target
	}
	return ""
}

type discussionRaw struct {
	ID    string    `json:"id"`
	Notes []noteRaw `json:"notes"`
}

type noteRaw struct {
	ID        int    `json:"id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	System    bool   `json:"system"`
	Resolved  bool   `json:"resolved"`
	Author    struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"author"`
	Position struct {
		NewPath string `json:"new_path"`
		OldPath string `json:"old_path"`
		NewLine *int   `json:"new_line"`
		OldLine *int   `json:"old_line"`
	} `json:"position"`
}

func noteToComment(threadID string, note noteRaw) *scm.Comment {
	author := note.Author.Name
	if author == "" {
		author = note.Author.Username
	}
	line := 0
	if note.Position.NewLine != nil {
		line = *note.Position.NewLine
	}
	file := note.Position.NewPath
	id := strconv.Itoa(note.ID)
	if threadID != "" {
		id = threadID + ":" + id
	}
	return &scm.Comment{
		ID:        id,
		ThreadID:  threadID,
		Body:      note.Body,
		File:      file,
		Line:      line,
		Author:    author,
		CreatedAt: note.CreatedAt,
		Resolved:  note.Resolved,
	}
}
