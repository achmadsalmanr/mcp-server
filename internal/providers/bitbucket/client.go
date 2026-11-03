package bitbucket

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
	workspace string
	username  string
	token     string
	apiBase   string
	http      *http.Client
}

type PullRequestRef struct {
	Workspace string
	RepoSlug  string
	ID        int
}

func NewClient(workspace, username, token, apiBase string) *Client {
	return &Client{
		workspace: workspace,
		username:  username,
		token:     token,
		apiBase:   strings.TrimRight(apiBase, "/"),
		http:      scm.NewHTTPClient(),
	}
}

func (c *Client) Provider() string { return scm.ProviderBitbucket }

func (c *Client) SupportsURL(rawURL string) bool {
	ref, err := ParseURL(rawURL)
	if err != nil {
		return false
	}
	return c.workspace == "" || ref.Workspace == c.workspace
}

func ParseURL(rawURL string) (PullRequestRef, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return PullRequestRef{}, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Host != "bitbucket.org" {
		return PullRequestRef{}, fmt.Errorf("URL host must be bitbucket.org, got %q", u.Host)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 || parts[2] != "pull-requests" {
		return PullRequestRef{}, fmt.Errorf("URL does not match https://bitbucket.org/{workspace}/{repo}/pull-requests/{id}: %q", rawURL)
	}
	id, err := strconv.Atoi(parts[3])
	if err != nil {
		return PullRequestRef{}, fmt.Errorf("pull request ID %q is not an integer", parts[3])
	}
	if parts[0] == "" || parts[1] == "" {
		return PullRequestRef{}, fmt.Errorf("workspace and repo slug must not be empty")
	}
	if !scm.ValidPathSegment(parts[0]) || !scm.ValidPathSegment(parts[1]) {
		return PullRequestRef{}, fmt.Errorf("invalid workspace or repo slug in URL: %q", rawURL)
	}
	return PullRequestRef{Workspace: parts[0], RepoSlug: parts[1], ID: id}, nil
}

func (c *Client) GetPullRequest(ctx context.Context, prURL string) (*scm.PullRequest, error) {
	ref, err := c.validateRef(prURL)
	if err != nil {
		return nil, err
	}
	var v struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		State       string `json:"state"`
		CreatedOn   string `json:"created_on"`
		Author      struct {
			DisplayName string `json:"display_name"`
			Nickname    string `json:"nickname"`
		} `json:"author"`
		Source struct {
			Branch struct{ Name string } `json:"branch"`
			Commit struct{ Hash string } `json:"commit"`
		} `json:"source"`
		Destination struct {
			Branch struct{ Name string } `json:"branch"`
		} `json:"destination"`
		Links struct {
			Self struct{ Href string } `json:"self"`
			HTML struct{ Href string } `json:"html"`
		} `json:"links"`
	}
	if err := c.do(ctx, http.MethodGet, c.prBase(ref), nil, &v); err != nil {
		return nil, fmt.Errorf("fetching pull request: %w", err)
	}
	author := v.Author.DisplayName
	if author == "" {
		author = v.Author.Nickname
	}
	return &scm.PullRequest{
		Provider:         c.Provider(),
		Repository:       ref.Workspace + "/" + ref.RepoSlug,
		ID:               v.ID,
		Title:            v.Title,
		Description:      v.Description,
		Author:           author,
		SourceBranch:     v.Source.Branch.Name,
		DestBranch:       v.Destination.Branch.Name,
		State:            v.State,
		CreatedAt:        v.CreatedOn,
		SourceCommitHash: v.Source.Commit.Hash,
		WebURL:           v.Links.HTML.Href,
		APIURL:           v.Links.Self.Href,
	}, nil
}

func (c *Client) GetPullRequestDiff(ctx context.Context, prURL string) (string, error) {
	ref, err := c.validateRef(prURL)
	if err != nil {
		return "", err
	}
	reqURL := c.apiBase + c.prBase(ref) + "/diff"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("building diff request: %w", err)
	}
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching diff: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, scm.MaxBodyRead))
		return "", &scm.APIError{Provider: c.Provider(), StatusCode: resp.StatusCode, Path: c.prBase(ref) + "/diff", Body: string(b)}
	}
	b, err := scm.ReadBody(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading diff response: %w", err)
	}
	return string(b), nil
}

func (c *Client) ListPullRequestCommits(ctx context.Context, prURL string) ([]scm.Commit, error) {
	ref, err := c.validateRef(prURL)
	if err != nil {
		return nil, err
	}
	type raw struct {
		Hash    string `json:"hash"`
		Message string `json:"message"`
		Author  struct {
			Raw string `json:"raw"`
		} `json:"author"`
		Links struct {
			HTML struct{ Href string } `json:"html"`
		} `json:"links"`
	}
	items, err := getAllPages[raw](ctx, c, c.prBase(ref)+"/commits")
	if err != nil {
		return nil, fmt.Errorf("fetching commits: %w", err)
	}
	commits := make([]scm.Commit, 0, len(items))
	for _, item := range items {
		hash := item.Hash
		if len(hash) > 12 {
			hash = hash[:12]
		}
		commits = append(commits, scm.Commit{
			Hash:    hash,
			Message: strings.SplitN(item.Message, "\n", 2)[0],
			Author:  item.Author.Raw,
			WebURL:  item.Links.HTML.Href,
		})
	}
	return commits, nil
}

func (c *Client) ListPullRequestComments(ctx context.Context, prURL string) ([]scm.Comment, error) {
	ref, err := c.validateRef(prURL)
	if err != nil {
		return nil, err
	}
	type raw struct {
		ID      int `json:"id"`
		Content struct {
			Raw string `json:"raw"`
		} `json:"content"`
		Inline *struct {
			Path string `json:"path"`
			To   int    `json:"to"`
		} `json:"inline"`
		Author struct {
			DisplayName string `json:"display_name"`
			Nickname    string `json:"nickname"`
		} `json:"author"`
		CreatedOn string `json:"created_on"`
		Deleted   bool   `json:"deleted"`
	}
	items, err := getAllPages[raw](ctx, c, c.prBase(ref)+"/comments")
	if err != nil {
		return nil, fmt.Errorf("fetching comments: %w", err)
	}
	comments := make([]scm.Comment, 0, len(items))
	for _, item := range items {
		if item.Deleted {
			continue
		}
		author := item.Author.DisplayName
		if author == "" {
			author = item.Author.Nickname
		}
		comment := scm.Comment{
			ID:        strconv.Itoa(item.ID),
			ThreadID:  strconv.Itoa(item.ID),
			Body:      item.Content.Raw,
			Author:    author,
			CreatedAt: item.CreatedOn,
		}
		if item.Inline != nil {
			comment.File = item.Inline.Path
			comment.Line = item.Inline.To
		}
		comments = append(comments, comment)
	}
	return comments, nil
}

func (c *Client) CreatePullRequestComment(ctx context.Context, prURL string, input scm.CreateCommentInput) (*scm.Comment, error) {
	ref, err := c.validateRef(prURL)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"content": map[string]string{"raw": input.Body}}
	if input.FilePath != "" {
		payload["inline"] = map[string]any{"to": input.NewLine, "path": input.FilePath}
	}
	var v struct {
		ID      int `json:"id"`
		Content struct {
			Raw string `json:"raw"`
		} `json:"content"`
		Inline *struct {
			Path string `json:"path"`
			To   int    `json:"to"`
		} `json:"inline"`
		Author struct {
			DisplayName string `json:"display_name"`
			Nickname    string `json:"nickname"`
		} `json:"author"`
		CreatedOn string `json:"created_on"`
	}
	if err := c.do(ctx, http.MethodPost, c.prBase(ref)+"/comments", payload, &v); err != nil {
		return nil, fmt.Errorf("creating comment: %w", err)
	}
	author := v.Author.DisplayName
	if author == "" {
		author = v.Author.Nickname
	}
	comment := &scm.Comment{
		ID:        strconv.Itoa(v.ID),
		ThreadID:  strconv.Itoa(v.ID),
		Body:      v.Content.Raw,
		Author:    author,
		CreatedAt: v.CreatedOn,
	}
	if v.Inline != nil {
		comment.File = v.Inline.Path
		comment.Line = v.Inline.To
	}
	return comment, nil
}

func (c *Client) ListPullRequestTasks(ctx context.Context, prURL string) ([]scm.Task, error) {
	ref, err := c.validateRef(prURL)
	if err != nil {
		return nil, err
	}
	type raw struct {
		ID      int    `json:"id"`
		State   string `json:"state"`
		Content struct {
			Raw string `json:"raw"`
		} `json:"content"`
		Creator struct {
			DisplayName string `json:"display_name"`
			Nickname    string `json:"nickname"`
		} `json:"creator"`
		CreatedOn  string `json:"created_on"`
		ResolvedOn string `json:"resolved_on"`
		Resolver   *struct {
			DisplayName string `json:"display_name"`
			Nickname    string `json:"nickname"`
		} `json:"resolver"`
	}
	items, err := getAllPages[raw](ctx, c, c.prBase(ref)+"/tasks")
	if err != nil {
		return nil, fmt.Errorf("fetching tasks: %w", err)
	}
	tasks := make([]scm.Task, 0, len(items))
	for _, item := range items {
		author := item.Creator.DisplayName
		if author == "" {
			author = item.Creator.Nickname
		}
		task := scm.Task{
			ID:         strconv.Itoa(item.ID),
			State:      item.State,
			Body:       item.Content.Raw,
			Author:     author,
			CreatedAt:  item.CreatedOn,
			ResolvedAt: item.ResolvedOn,
		}
		if item.Resolver != nil {
			task.ResolvedBy = item.Resolver.DisplayName
			if task.ResolvedBy == "" {
				task.ResolvedBy = item.Resolver.Nickname
			}
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (c *Client) CreatePullRequestTask(ctx context.Context, prURL string, input scm.CreateTaskInput) (*scm.Task, error) {
	ref, err := c.validateRef(prURL)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"content": map[string]string{"raw": input.Body}}
	if input.CommentID != "" {
		if id, err := strconv.Atoi(input.CommentID); err == nil {
			payload["comment"] = map[string]int{"id": id}
		}
	}
	var v struct {
		ID      int    `json:"id"`
		State   string `json:"state"`
		Content struct {
			Raw string `json:"raw"`
		} `json:"content"`
		Creator struct {
			DisplayName string `json:"display_name"`
			Nickname    string `json:"nickname"`
		} `json:"creator"`
		CreatedOn string `json:"created_on"`
	}
	if err := c.do(ctx, http.MethodPost, c.prBase(ref)+"/tasks", payload, &v); err != nil {
		return nil, fmt.Errorf("creating task: %w", err)
	}
	author := v.Creator.DisplayName
	if author == "" {
		author = v.Creator.Nickname
	}
	return &scm.Task{ID: strconv.Itoa(v.ID), State: v.State, Body: v.Content.Raw, Author: author, CreatedAt: v.CreatedOn}, nil
}

func (c *Client) UpdatePullRequestTask(ctx context.Context, prURL string, taskID string, input scm.UpdateTaskInput) (*scm.Task, error) {
	ref, err := c.validateRef(prURL)
	if err != nil {
		return nil, err
	}
	id, err := strconv.Atoi(taskID)
	if err != nil {
		return nil, fmt.Errorf("task_id must be an integer for Bitbucket: %w", err)
	}
	var v struct {
		ID      int    `json:"id"`
		State   string `json:"state"`
		Content struct {
			Raw string `json:"raw"`
		} `json:"content"`
	}
	path := fmt.Sprintf("%s/tasks/%d", c.prBase(ref), id)
	if err := c.do(ctx, http.MethodPut, path, map[string]string{"state": input.State}, &v); err != nil {
		return nil, fmt.Errorf("updating task: %w", err)
	}
	return &scm.Task{ID: strconv.Itoa(v.ID), State: v.State, Body: v.Content.Raw}, nil
}

func (c *Client) ListPullRequestStatuses(ctx context.Context, prURL string) ([]scm.Status, error) {
	ref, err := c.validateRef(prURL)
	if err != nil {
		return nil, err
	}
	type raw struct {
		Key       string `json:"key"`
		Name      string `json:"name"`
		State     string `json:"state"`
		URL       string `json:"url"`
		CreatedOn string `json:"created_on"`
		UpdatedOn string `json:"updated_on"`
	}
	items, err := getAllPages[raw](ctx, c, c.prBase(ref)+"/statuses")
	if err != nil {
		return nil, fmt.Errorf("fetching statuses: %w", err)
	}
	statuses := make([]scm.Status, 0, len(items))
	for _, item := range items {
		statuses = append(statuses, scm.Status{
			Key:       item.Key,
			Name:      item.Name,
			State:     item.State,
			URL:       item.URL,
			CreatedAt: item.CreatedOn,
			UpdatedAt: item.UpdatedOn,
		})
	}
	return statuses, nil
}

func (c *Client) ApprovePullRequest(ctx context.Context, prURL string) error {
	ref, err := c.validateRef(prURL)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, c.prBase(ref)+"/approve", nil, nil)
}

func (c *Client) RequestChanges(ctx context.Context, prURL string) error {
	ref, err := c.validateRef(prURL)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, c.prBase(ref)+"/request-changes", nil, nil)
}

func (c *Client) ResolveCommentThread(ctx context.Context, prURL string, commentID string) error {
	ref, id, err := c.refAndCommentID(prURL, commentID)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, fmt.Sprintf("%s/comments/%d/resolve", c.prBase(ref), id), nil, nil)
}

func (c *Client) ReopenCommentThread(ctx context.Context, prURL string, commentID string) error {
	ref, id, err := c.refAndCommentID(prURL, commentID)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("%s/comments/%d/resolve", c.prBase(ref), id), nil, nil)
}

func (c *Client) validateRef(prURL string) (PullRequestRef, error) {
	ref, err := ParseURL(prURL)
	if err != nil {
		return PullRequestRef{}, err
	}
	if c.workspace != "" && ref.Workspace != c.workspace {
		return PullRequestRef{}, fmt.Errorf("pull request workspace %q does not match configured workspace %q", ref.Workspace, c.workspace)
	}
	return ref, nil
}

func (c *Client) prBase(ref PullRequestRef) string {
	return fmt.Sprintf("/repositories/%s/%s/pullrequests/%d", ref.Workspace, ref.RepoSlug, ref.ID)
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	reqURL, err := scm.ResolveURL(c.apiBase, path)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	c.authorize(req)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
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

func (c *Client) authorize(req *http.Request) {
	if c.username != "" || c.token != "" {
		req.SetBasicAuth(c.username, c.token)
	}
}

type page[T any] struct {
	Values []T    `json:"values"`
	Next   string `json:"next"`
}

func getAllPages[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var result []T
	seen := map[string]bool{}
	next := path
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
		var p page[T]
		if err := c.do(ctx, http.MethodGet, next, nil, &p); err != nil {
			return nil, err
		}
		result = append(result, p.Values...)
		next = p.Next
	}
	return result, nil
}

func (c *Client) refAndCommentID(prURL, commentID string) (PullRequestRef, int, error) {
	ref, err := c.validateRef(prURL)
	if err != nil {
		return PullRequestRef{}, 0, err
	}
	commentID = strings.Split(commentID, ":")[0]
	id, err := strconv.Atoi(commentID)
	if err != nil {
		return PullRequestRef{}, 0, fmt.Errorf("comment_id must be an integer for Bitbucket: %w", err)
	}
	return ref, id, nil
}
