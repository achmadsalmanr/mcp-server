package scm

import (
	"context"
	"errors"
)

const (
	ProviderBitbucket = "bitbucket"
	ProviderGitHub    = "github"
	ProviderGitLab    = "gitlab"
)

var ErrUnsupported = errors.New("unsupported operation")

type PullRequestStore interface {
	Provider() string
	SupportsURL(rawURL string) bool
	GetPullRequest(ctx context.Context, prURL string) (*PullRequest, error)
	GetPullRequestDiff(ctx context.Context, prURL string) (string, error)
	ListPullRequestCommits(ctx context.Context, prURL string) ([]Commit, error)
	ListPullRequestComments(ctx context.Context, prURL string) ([]Comment, error)
	CreatePullRequestComment(ctx context.Context, prURL string, input CreateCommentInput) (*Comment, error)
	ListPullRequestTasks(ctx context.Context, prURL string) ([]Task, error)
	CreatePullRequestTask(ctx context.Context, prURL string, input CreateTaskInput) (*Task, error)
	UpdatePullRequestTask(ctx context.Context, prURL string, taskID string, input UpdateTaskInput) (*Task, error)
	ListPullRequestStatuses(ctx context.Context, prURL string) ([]Status, error)
	ApprovePullRequest(ctx context.Context, prURL string) error
	RequestChanges(ctx context.Context, prURL string) error
	ResolveCommentThread(ctx context.Context, prURL string, commentID string) error
	ReopenCommentThread(ctx context.Context, prURL string, commentID string) error
}

type PullRequest struct {
	Provider         string `json:"provider"`
	Repository       string `json:"repository"`
	ID               int    `json:"id"`
	Title            string `json:"title"`
	Description      string `json:"description,omitempty"`
	Author           string `json:"author"`
	SourceBranch     string `json:"source_branch"`
	DestBranch       string `json:"dest_branch"`
	State            string `json:"state"`
	CreatedAt        string `json:"created_at,omitempty"`
	SourceCommitHash string `json:"source_commit_hash"`
	WebURL           string `json:"web_url,omitempty"`
	APIURL           string `json:"api_url,omitempty"`
}

type Commit struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Author  string `json:"author,omitempty"`
	WebURL  string `json:"web_url,omitempty"`
}

type Comment struct {
	ID        string `json:"id"`
	ThreadID  string `json:"thread_id,omitempty"`
	Body      string `json:"body"`
	File      string `json:"file,omitempty"`
	Line      int    `json:"line,omitempty"`
	Author    string `json:"author"`
	CreatedAt string `json:"created_at,omitempty"`
	Resolved  bool   `json:"resolved,omitempty"`
}

type Task struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	Body       string `json:"body"`
	Author     string `json:"author,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	ResolvedAt string `json:"resolved_at,omitempty"`
	ResolvedBy string `json:"resolved_by,omitempty"`
}

type Status struct {
	Key       string `json:"key"`
	Name      string `json:"name,omitempty"`
	State     string `json:"state"`
	URL       string `json:"url,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type StatusesSummary struct {
	Total          int      `json:"total"`
	Successful     int      `json:"successful"`
	Failed         int      `json:"failed"`
	InProgress     int      `json:"in_progress"`
	Pending        int      `json:"pending"`
	Unknown        int      `json:"unknown"`
	FailedStatuses []Status `json:"failed_statuses,omitempty"`
}

type CreateCommentInput struct {
	Body       string
	FilePath   string
	NewLine    int
	OldLine    int
	CommitHash string
}

type CreateTaskInput struct {
	Body      string
	CommentID string
}

type UpdateTaskInput struct {
	State string
}

func IsUnsupported(err error) bool {
	return errors.Is(err, ErrUnsupported)
}
