package github

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

	"github.com/achmadsalmanr/mcp-server/internal/scm"
)

const maxPaginationPages = 100

type Client struct {
	token      string
	apiBase    string
	webBase    string
	webHost    string
	apiVersion string
	http       *http.Client
}

type PullRequestRef struct {
	Owner  string
	Repo   string
	Number int
}

func NewClient(token, apiBase, webBase, apiVersion string) *Client {
	webBase = strings.TrimRight(webBase, "/")
	webHost := "github.com"
	if u, err := url.Parse(webBase); err == nil && u.Host != "" {
		webHost = u.Host
	}
	return &Client{
		token:      token,
		apiBase:    strings.TrimRight(apiBase, "/"),
		webBase:    webBase,
		webHost:    webHost,
		apiVersion: apiVersion,
		http:       scm.NewHTTPClient(),
	}
}

func (c *Client) Provider() string { return scm.ProviderGitHub }

func (c *Client) SupportsURL(rawURL string) bool {
	_, err := c.parseURL(rawURL)
	return err == nil
}

func (c *Client) parseURL(rawURL string) (PullRequestRef, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return PullRequestRef{}, fmt.Errorf("invalid URL: %w", err)
	}
	if !strings.EqualFold(u.Host, c.webHost) {
		return PullRequestRef{}, fmt.Errorf("URL host must be %s, got %q", c.webHost, u.Host)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 || parts[2] != "pull" {
		return PullRequestRef{}, fmt.Errorf("URL does not match %s/{owner}/{repo}/pull/{number}: %q", c.webBase, rawURL)
	}
	if !scm.ValidPathSegment(parts[0]) || !scm.ValidPathSegment(parts[1]) {
		return PullRequestRef{}, fmt.Errorf("invalid owner or repository name in URL: %q", rawURL)
	}
	number, err := strconv.Atoi(parts[3])
	if err != nil {
		return PullRequestRef{}, fmt.Errorf("pull request number %q is not an integer", parts[3])
	}
	return PullRequestRef{Owner: parts[0], Repo: parts[1], Number: number}, nil
}

func (c *Client) GetPullRequest(ctx context.Context, prURL string) (*scm.PullRequest, error) {
	ref, err := c.parseURL(prURL)
	if err != nil {
		return nil, err
	}
	var v struct {
		ID     int64  `json:"id"`
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		State  string `json:"state"`
		HTML   string `json:"html_url"`
		URL    string `json:"url"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
		Head struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		CreatedAt string `json:"created_at"`
	}
	if err := c.doJSON(ctx, http.MethodGet, c.prBase(ref), nil, &v); err != nil {
		return nil, fmt.Errorf("fetching pull request: %w", err)
	}
	return &scm.PullRequest{
		Provider:         c.Provider(),
		Repository:       ref.Owner + "/" + ref.Repo,
		ID:               v.Number,
		Title:            v.Title,
		Description:      v.Body,
		Author:           v.User.Login,
		SourceBranch:     v.Head.Ref,
		DestBranch:       v.Base.Ref,
		State:            v.State,
		CreatedAt:        v.CreatedAt,
		SourceCommitHash: v.Head.SHA,
		WebURL:           v.HTML,
		APIURL:           v.URL,
	}, nil
}

func (c *Client) GetPullRequestDiff(ctx context.Context, prURL string) (string, error) {
	ref, err := c.parseURL(prURL)
	if err != nil {
		return "", err
	}
	return c.doRaw(ctx, http.MethodGet, c.prBase(ref), "application/vnd.github.v3.diff")
}

func (c *Client) ListPullRequestCommits(ctx context.Context, prURL string) ([]scm.Commit, error) {
	ref, err := c.parseURL(prURL)
	if err != nil {
		return nil, err
	}
	type raw struct {
		SHA    string `json:"sha"`
		HTML   string `json:"html_url"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"commit"`
	}
	items, err := getAllPages[raw](ctx, c, c.prBase(ref)+"/commits")
	if err != nil {
		return nil, fmt.Errorf("fetching commits: %w", err)
	}
	commits := make([]scm.Commit, 0, len(items))
	for _, item := range items {
		hash := item.SHA
		if len(hash) > 12 {
			hash = hash[:12]
		}
		commits = append(commits, scm.Commit{
			Hash:    hash,
			Message: strings.SplitN(item.Commit.Message, "\n", 2)[0],
			Author:  item.Commit.Author.Name,
			WebURL:  item.HTML,
		})
	}
	return commits, nil
}

func (c *Client) ListPullRequestComments(ctx context.Context, prURL string) ([]scm.Comment, error) {
	ref, err := c.parseURL(prURL)
	if err != nil {
		return nil, err
	}
	type reviewComment struct {
		ID           int64  `json:"id"`
		Body         string `json:"body"`
		Path         string `json:"path"`
		Line         *int   `json:"line"`
		OriginalLine *int   `json:"original_line"`
		User         struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt string `json:"created_at"`
	}
	reviewComments, err := getAllPages[reviewComment](ctx, c, c.prBase(ref)+"/comments")
	if err != nil {
		return nil, fmt.Errorf("fetching review comments: %w", err)
	}
	comments := make([]scm.Comment, 0, len(reviewComments))
	for _, item := range reviewComments {
		line := 0
		if item.Line != nil {
			line = *item.Line
		} else if item.OriginalLine != nil {
			line = *item.OriginalLine
		}
		id := strconv.FormatInt(item.ID, 10)
		comments = append(comments, scm.Comment{
			ID:        id,
			ThreadID:  id,
			Body:      item.Body,
			File:      item.Path,
			Line:      line,
			Author:    item.User.Login,
			CreatedAt: item.CreatedAt,
		})
	}

	type issueComment struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt string `json:"created_at"`
	}
	issueComments, err := getAllPages[issueComment](ctx, c, fmt.Sprintf("/repos/%s/%s/issues/%d/comments", ref.Owner, ref.Repo, ref.Number))
	if err != nil {
		return nil, fmt.Errorf("fetching issue comments: %w", err)
	}
	for _, item := range issueComments {
		id := strconv.FormatInt(item.ID, 10)
		comments = append(comments, scm.Comment{ID: id, ThreadID: id, Body: item.Body, Author: item.User.Login, CreatedAt: item.CreatedAt})
	}
	return comments, nil
}

func (c *Client) CreatePullRequestComment(ctx context.Context, prURL string, input scm.CreateCommentInput) (*scm.Comment, error) {
	ref, err := c.parseURL(prURL)
	if err != nil {
		return nil, err
	}
	if input.FilePath == "" {
		var out struct {
			ID   int64  `json:"id"`
			Body string `json:"body"`
			User struct {
				Login string `json:"login"`
			} `json:"user"`
			CreatedAt string `json:"created_at"`
		}
		path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", ref.Owner, ref.Repo, ref.Number)
		if err := c.doJSON(ctx, http.MethodPost, path, map[string]string{"body": input.Body}, &out); err != nil {
			return nil, fmt.Errorf("creating issue comment: %w", err)
		}
		id := strconv.FormatInt(out.ID, 10)
		return &scm.Comment{ID: id, ThreadID: id, Body: out.Body, Author: out.User.Login, CreatedAt: out.CreatedAt}, nil
	}
	commitID := input.CommitHash
	if commitID == "" {
		pr, err := c.GetPullRequest(ctx, prURL)
		if err != nil {
			return nil, err
		}
		commitID = pr.SourceCommitHash
	}
	payload := map[string]any{
		"body":      input.Body,
		"commit_id": commitID,
		"path":      input.FilePath,
		"line":      input.NewLine,
		"side":      "RIGHT",
	}
	var out struct {
		ID        int64                  `json:"id"`
		Body      string                 `json:"body"`
		Path      string                 `json:"path"`
		Line      *int                   `json:"line"`
		User      struct{ Login string } `json:"user"`
		CreatedAt string                 `json:"created_at"`
	}
	if err := c.doJSON(ctx, http.MethodPost, c.prBase(ref)+"/comments", payload, &out); err != nil {
		return nil, fmt.Errorf("creating review comment: %w", err)
	}
	line := input.NewLine
	if out.Line != nil {
		line = *out.Line
	}
	id := strconv.FormatInt(out.ID, 10)
	return &scm.Comment{ID: id, ThreadID: id, Body: out.Body, File: out.Path, Line: line, Author: out.User.Login, CreatedAt: out.CreatedAt}, nil
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
	ref, err := c.parseURL(prURL)
	if err != nil {
		return nil, err
	}
	pr, err := c.GetPullRequest(ctx, prURL)
	if err != nil {
		return nil, err
	}
	var out struct {
		Statuses []struct {
			Context     string `json:"context"`
			Description string `json:"description"`
			State       string `json:"state"`
			TargetURL   string `json:"target_url"`
			CreatedAt   string `json:"created_at"`
			UpdatedAt   string `json:"updated_at"`
		} `json:"statuses"`
	}
	path := fmt.Sprintf("/repos/%s/%s/commits/%s/status", ref.Owner, ref.Repo, pr.SourceCommitHash)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, fmt.Errorf("fetching statuses: %w", err)
	}
	statuses := make([]scm.Status, 0, len(out.Statuses))
	for _, item := range out.Statuses {
		statuses = append(statuses, scm.Status{
			Key:       item.Context,
			Name:      item.Description,
			State:     item.State,
			URL:       item.TargetURL,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}
	return statuses, nil
}

func (c *Client) ApprovePullRequest(ctx context.Context, prURL string) error {
	ref, err := c.parseURL(prURL)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodPost, c.prBase(ref)+"/reviews", map[string]string{"event": "APPROVE"}, nil)
}

func (c *Client) RequestChanges(ctx context.Context, prURL string) error {
	ref, err := c.parseURL(prURL)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodPost, c.prBase(ref)+"/reviews", map[string]string{
		"event": "REQUEST_CHANGES",
		"body":  "Changes requested.",
	}, nil)
}

func (c *Client) ResolveCommentThread(context.Context, string, string) error {
	return scm.ErrUnsupported
}

func (c *Client) ReopenCommentThread(context.Context, string, string) error {
	return scm.ErrUnsupported
}

func (c *Client) prBase(ref PullRequestRef) string {
	return fmt.Sprintf("/repos/%s/%s/pulls/%d", ref.Owner, ref.Repo, ref.Number)
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	return c.do(ctx, method, path, body, out, "application/vnd.github+json")
}

func (c *Client) doRaw(ctx context.Context, method, path, accept string) (string, error) {
	req, err := c.newRequest(ctx, method, path, nil, accept)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, scm.MaxBodyRead))
		return "", &scm.APIError{Provider: c.Provider(), StatusCode: resp.StatusCode, Path: path, Body: string(b)}
	}
	b, err := scm.ReadBody(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response from %s: %w", path, err)
	}
	return string(b), nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any, accept string) error {
	req, err := c.newRequest(ctx, method, path, body, accept)
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
		if err := json.NewDecoder(scm.LimitBody(resp.Body)).Decode(out); err != nil {
			return fmt.Errorf("decoding response from %s: %w", path, err)
		}
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body any, accept string) (*http.Request, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	reqURL, err := scm.ResolveURL(c.apiBase, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", accept)
	if c.apiVersion != "" {
		req.Header.Set("X-GitHub-Api-Version", c.apiVersion)
	}
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
		req, err := c.newRequest(ctx, http.MethodGet, next, nil, "application/vnd.github+json")
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
		if err := json.NewDecoder(scm.LimitBody(resp.Body)).Decode(&page); err != nil {
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
