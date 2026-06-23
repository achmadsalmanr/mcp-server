package review

import (
	"github.com/achmadsalmanr/mcp-server/internal/diff"
	"github.com/achmadsalmanr/mcp-server/internal/scm"
)

type GetPRContextInput struct {
	Provider     string
	PRURL        string
	MaxDiffLines int
	MaxFiles     int
	DiffCursor   string
}

type GetPRDiffInput struct {
	Provider string
	PRURL    string
	File     string
	Cursor   string
	MaxLines int
	MaxFiles int
}

type DraftReviewInput struct {
	Provider       string
	PRURL          string
	Findings       []ReviewFinding
	SummaryBody    string
	SummaryVerdict string
}

type ApprovePRInput struct {
	Provider                 string
	PRURL                    string
	ExpectedSourceCommitHash string
}

type RequestChangesInput struct {
	Provider                 string
	PRURL                    string
	ExpectedSourceCommitHash string
}

type ResolveTaskInput struct {
	Provider string
	PRURL    string
	TaskID   string
}

type ReopenTaskInput struct {
	Provider string
	PRURL    string
	TaskID   string
}

type ResolveCommentThreadInput struct {
	Provider  string
	PRURL     string
	CommentID string
}

type ReopenCommentThreadInput struct {
	Provider  string
	PRURL     string
	CommentID string
}

type ReviewFinding struct {
	File       string `json:"file"`
	NewLineNo  int    `json:"new_line_no"`
	Body       string `json:"body"`
	Severity   string `json:"severity,omitempty"`
	CreateTask bool   `json:"create_task,omitempty"`
}

type ReviewDraft struct {
	Provider                string             `json:"provider"`
	PRURL                   string             `json:"pr_url"`
	SourceCommitHash        string             `json:"source_commit_hash"`
	CommentsToPost          []DraftComment     `json:"comments_to_post"`
	TasksToCreate           []DraftTask        `json:"tasks_to_create,omitempty"`
	SummaryComment          *DraftSummary      `json:"summary_comment,omitempty"`
	Skipped                 []SkippedFinding   `json:"skipped,omitempty"`
	Duplicates              []DuplicateFinding `json:"duplicates,omitempty"`
	ExistingCommentsChecked bool               `json:"existing_comments_checked"`
}

type DraftComment struct {
	File      string `json:"file"`
	NewLineNo int    `json:"new_line_no"`
	Body      string `json:"body"`
	Severity  string `json:"severity,omitempty"`
}

type DraftTask struct {
	CommentIndex int    `json:"comment_index"`
	Body         string `json:"body"`
}

type DraftSummary struct {
	Body    string `json:"body"`
	Verdict string `json:"verdict"`
}

type SkippedFinding struct {
	Index   int           `json:"index"`
	Reason  string        `json:"reason"`
	Finding ReviewFinding `json:"finding"`
}

type DuplicateFinding struct {
	Index   int           `json:"index"`
	Reason  string        `json:"reason"`
	Finding ReviewFinding `json:"finding"`
}

type PRContext struct {
	Provider        string              `json:"provider"`
	PR              *scm.PullRequest    `json:"pr"`
	Commits         []scm.Commit        `json:"commits"`
	Comments        []scm.Comment       `json:"comments"`
	Tasks           []scm.Task          `json:"tasks,omitempty"`
	StatusesSummary scm.StatusesSummary `json:"statuses_summary"`
	Diff            *diff.DiffPage      `json:"diff"`
	Warnings        []string            `json:"warnings,omitempty"`
}

type PostReviewResult struct {
	PostedComments []scm.Comment `json:"posted_comments"`
	CreatedTasks   []scm.Task    `json:"created_tasks,omitempty"`
	PostedSummary  *scm.Comment  `json:"posted_summary,omitempty"`
	Failed         []PostFailure `json:"failed,omitempty"`
}

type PostFailure struct {
	Kind  string `json:"kind"`
	Index int    `json:"index"`
	Error string `json:"error"`
}

type ApprovePRResult struct {
	Approved         bool   `json:"approved"`
	SourceCommitHash string `json:"source_commit_hash"`
}

type RequestChangesResult struct {
	RequestedChanges bool   `json:"requested_changes"`
	SourceCommitHash string `json:"source_commit_hash"`
}

type TaskResult struct {
	TaskID string `json:"task_id"`
	State  string `json:"state"`
}
