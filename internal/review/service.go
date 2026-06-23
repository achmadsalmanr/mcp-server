package review

import (
	"context"
	"fmt"
	"strings"

	"github.com/achmadsalmanr/mcp-server/internal/diff"
	"github.com/achmadsalmanr/mcp-server/internal/scm"
)

var validSeverities = map[string]bool{"HIGH": true, "MEDIUM": true, "LOW": true}
var validVerdicts = map[string]bool{"PASS": true, "COMMENT": true, "BLOCK": true}

type Service struct {
	stores []scm.PullRequestStore
}

func NewService(stores ...scm.PullRequestStore) *Service {
	return &Service{stores: stores}
}

func (s *Service) Providers() []string {
	out := make([]string, 0, len(s.stores))
	for _, store := range s.stores {
		out = append(out, store.Provider())
	}
	return out
}

func (s *Service) GetPRContext(ctx context.Context, input GetPRContextInput) (*PRContext, error) {
	store, err := s.resolve(input.Provider, input.PRURL)
	if err != nil {
		return nil, err
	}
	pr, err := store.GetPullRequest(ctx, input.PRURL)
	if err != nil {
		return nil, fmt.Errorf("get change request: %w", err)
	}
	commits, err := store.ListPullRequestCommits(ctx, input.PRURL)
	if err != nil {
		return nil, fmt.Errorf("list commits: %w", err)
	}
	comments, err := store.ListPullRequestComments(ctx, input.PRURL)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	tasks, err := store.ListPullRequestTasks(ctx, input.PRURL)
	if err != nil && !scm.IsUnsupported(err) {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	statuses, err := store.ListPullRequestStatuses(ctx, input.PRURL)
	var warnings []string
	if err != nil {
		if !scm.IsUnsupported(err) {
			warnings = append(warnings, "statuses unavailable: "+err.Error())
		}
		statuses = nil
	}
	diffPage, err := s.diffPage(ctx, store, input.PRURL, input.DiffCursor, "", input.MaxDiffLines, input.MaxFiles)
	if err != nil {
		return nil, err
	}
	return &PRContext{
		Provider:        store.Provider(),
		PR:              pr,
		Commits:         commits,
		Comments:        comments,
		Tasks:           tasks,
		StatusesSummary: buildStatusesSummary(statuses),
		Diff:            diffPage,
		Warnings:        warnings,
	}, nil
}

func (s *Service) GetPRDiff(ctx context.Context, input GetPRDiffInput) (*diff.DiffPage, error) {
	store, err := s.resolve(input.Provider, input.PRURL)
	if err != nil {
		return nil, err
	}
	return s.diffPage(ctx, store, input.PRURL, input.Cursor, input.File, input.MaxLines, input.MaxFiles)
}

func (s *Service) DraftReviewComments(ctx context.Context, input DraftReviewInput) (*ReviewDraft, error) {
	store, err := s.resolve(input.Provider, input.PRURL)
	if err != nil {
		return nil, err
	}
	if input.SummaryVerdict != "" && !validVerdicts[input.SummaryVerdict] {
		return nil, fmt.Errorf("invalid summary_verdict %q: must be PASS, COMMENT, or BLOCK", input.SummaryVerdict)
	}
	for i, f := range input.Findings {
		if f.Severity != "" && !validSeverities[f.Severity] {
			return nil, fmt.Errorf("finding[%d]: invalid severity %q: must be HIGH, MEDIUM, or LOW", i, f.Severity)
		}
	}

	pr, err := store.GetPullRequest(ctx, input.PRURL)
	if err != nil {
		return nil, fmt.Errorf("get change request: %w", err)
	}
	rawDiff, err := store.GetPullRequestDiff(ctx, input.PRURL)
	if err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}
	parsed, err := diff.Parse(rawDiff)
	if err != nil {
		return nil, fmt.Errorf("parse diff: %w", err)
	}
	existing, err := store.ListPullRequestComments(ctx, input.PRURL)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}

	draft := &ReviewDraft{
		Provider:                store.Provider(),
		PRURL:                   input.PRURL,
		SourceCommitHash:        pr.SourceCommitHash,
		ExistingCommentsChecked: true,
	}
	if input.SummaryBody != "" {
		verdict := input.SummaryVerdict
		if verdict == "" {
			verdict = "COMMENT"
		}
		draft.SummaryComment = &DraftSummary{Body: input.SummaryBody, Verdict: verdict}
	}

	for i, finding := range input.Findings {
		if strings.TrimSpace(finding.Body) == "" {
			draft.Skipped = append(draft.Skipped, SkippedFinding{Index: i, Reason: "empty_body", Finding: finding})
			continue
		}
		line, err := parsed.FindLine(finding.File, finding.NewLineNo)
		if err != nil {
			draft.Skipped = append(draft.Skipped, SkippedFinding{Index: i, Reason: "line_not_in_diff", Finding: finding})
			continue
		}
		if line.Type == diff.LineRemoved {
			draft.Skipped = append(draft.Skipped, SkippedFinding{Index: i, Reason: "removed_line_cannot_be_anchor", Finding: finding})
			continue
		}
		if isDuplicate(existing, finding.File, finding.NewLineNo, normalizeBody(finding.Body)) {
			draft.Duplicates = append(draft.Duplicates, DuplicateFinding{Index: i, Reason: "same_file_line_body", Finding: finding})
			continue
		}
		commentIndex := len(draft.CommentsToPost)
		draft.CommentsToPost = append(draft.CommentsToPost, DraftComment{
			File:      finding.File,
			NewLineNo: finding.NewLineNo,
			Body:      finding.Body,
			Severity:  finding.Severity,
		})
		if finding.CreateTask {
			draft.TasksToCreate = append(draft.TasksToCreate, DraftTask{CommentIndex: commentIndex, Body: finding.Body})
		}
	}
	return draft, nil
}

func (s *Service) PostReviewComments(ctx context.Context, draft ReviewDraft) (*PostReviewResult, error) {
	store, err := s.resolve(draft.Provider, draft.PRURL)
	if err != nil {
		return nil, err
	}
	pr, err := store.GetPullRequest(ctx, draft.PRURL)
	if err != nil {
		return nil, fmt.Errorf("get change request for stale check: %w", err)
	}
	if pr.SourceCommitHash != draft.SourceCommitHash {
		return nil, fmt.Errorf("source commit changed: expected %s, got %s", draft.SourceCommitHash, pr.SourceCommitHash)
	}
	result := &PostReviewResult{}
	commentIDs := map[int]string{}
	for i, dc := range draft.CommentsToPost {
		comment, err := store.CreatePullRequestComment(ctx, draft.PRURL, scm.CreateCommentInput{
			Body:       dc.Body,
			FilePath:   dc.File,
			NewLine:    dc.NewLineNo,
			CommitHash: draft.SourceCommitHash,
		})
		if err != nil {
			result.Failed = append(result.Failed, PostFailure{Kind: "comment", Index: i, Error: err.Error()})
			continue
		}
		commentIDs[i] = comment.ID
		result.PostedComments = append(result.PostedComments, *comment)
	}
	for _, task := range draft.TasksToCreate {
		created, err := store.CreatePullRequestTask(ctx, draft.PRURL, scm.CreateTaskInput{
			Body:      task.Body,
			CommentID: commentIDs[task.CommentIndex],
		})
		if err != nil {
			result.Failed = append(result.Failed, PostFailure{Kind: "task", Index: task.CommentIndex, Error: err.Error()})
			continue
		}
		result.CreatedTasks = append(result.CreatedTasks, *created)
	}
	if draft.SummaryComment != nil {
		comment, err := store.CreatePullRequestComment(ctx, draft.PRURL, scm.CreateCommentInput{Body: draft.SummaryComment.Body})
		if err != nil {
			result.Failed = append(result.Failed, PostFailure{Kind: "summary", Index: 0, Error: err.Error()})
		} else {
			result.PostedSummary = comment
		}
	}
	return result, nil
}

func (s *Service) ApprovePR(ctx context.Context, input ApprovePRInput) (*ApprovePRResult, error) {
	store, err := s.resolve(input.Provider, input.PRURL)
	if err != nil {
		return nil, err
	}
	pr, err := store.GetPullRequest(ctx, input.PRURL)
	if err != nil {
		return nil, fmt.Errorf("get change request: %w", err)
	}
	if pr.SourceCommitHash != input.ExpectedSourceCommitHash {
		return nil, fmt.Errorf("source commit changed: expected %s, got %s", input.ExpectedSourceCommitHash, pr.SourceCommitHash)
	}
	if err := store.ApprovePullRequest(ctx, input.PRURL); err != nil {
		return nil, fmt.Errorf("approve change request: %w", err)
	}
	return &ApprovePRResult{Approved: true, SourceCommitHash: pr.SourceCommitHash}, nil
}

func (s *Service) RequestChangesPR(ctx context.Context, input RequestChangesInput) (*RequestChangesResult, error) {
	store, err := s.resolve(input.Provider, input.PRURL)
	if err != nil {
		return nil, err
	}
	pr, err := store.GetPullRequest(ctx, input.PRURL)
	if err != nil {
		return nil, fmt.Errorf("get change request: %w", err)
	}
	if pr.SourceCommitHash != input.ExpectedSourceCommitHash {
		return nil, fmt.Errorf("source commit changed: expected %s, got %s", input.ExpectedSourceCommitHash, pr.SourceCommitHash)
	}
	if err := store.RequestChanges(ctx, input.PRURL); err != nil {
		return nil, fmt.Errorf("request changes: %w", err)
	}
	return &RequestChangesResult{RequestedChanges: true, SourceCommitHash: pr.SourceCommitHash}, nil
}

func (s *Service) ResolveTask(ctx context.Context, input ResolveTaskInput) (*TaskResult, error) {
	store, err := s.resolve(input.Provider, input.PRURL)
	if err != nil {
		return nil, err
	}
	task, err := store.UpdatePullRequestTask(ctx, input.PRURL, input.TaskID, scm.UpdateTaskInput{State: "RESOLVED"})
	if err != nil {
		return nil, fmt.Errorf("resolve task: %w", err)
	}
	return &TaskResult{TaskID: task.ID, State: task.State}, nil
}

func (s *Service) ReopenTask(ctx context.Context, input ReopenTaskInput) (*TaskResult, error) {
	store, err := s.resolve(input.Provider, input.PRURL)
	if err != nil {
		return nil, err
	}
	task, err := store.UpdatePullRequestTask(ctx, input.PRURL, input.TaskID, scm.UpdateTaskInput{State: "UNRESOLVED"})
	if err != nil {
		return nil, fmt.Errorf("reopen task: %w", err)
	}
	return &TaskResult{TaskID: task.ID, State: task.State}, nil
}

func (s *Service) ResolveCommentThread(ctx context.Context, input ResolveCommentThreadInput) error {
	store, err := s.resolve(input.Provider, input.PRURL)
	if err != nil {
		return err
	}
	return store.ResolveCommentThread(ctx, input.PRURL, input.CommentID)
}

func (s *Service) ReopenCommentThread(ctx context.Context, input ReopenCommentThreadInput) error {
	store, err := s.resolve(input.Provider, input.PRURL)
	if err != nil {
		return err
	}
	return store.ReopenCommentThread(ctx, input.PRURL, input.CommentID)
}

func (s *Service) diffPage(ctx context.Context, store scm.PullRequestStore, prURL, cursor, file string, maxLines, maxFiles int) (*diff.DiffPage, error) {
	rawDiff, err := store.GetPullRequestDiff(ctx, prURL)
	if err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}
	parsed, err := diff.Parse(rawDiff)
	if err != nil {
		return nil, fmt.Errorf("parse diff: %w", err)
	}
	if maxLines == 0 {
		maxLines = 500
	}
	page, err := parsed.GetPage(diff.PageInput{PRURL: prURL, FileFilter: file, Cursor: cursor, MaxLines: maxLines, MaxFiles: maxFiles})
	if err != nil {
		return nil, fmt.Errorf("paginate diff: %w", err)
	}
	return page, nil
}

func (s *Service) resolve(provider, prURL string) (scm.PullRequestStore, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != "" {
		for _, store := range s.stores {
			if store.Provider() == provider {
				if !store.SupportsURL(prURL) {
					return nil, fmt.Errorf("%s client does not support URL %q", provider, prURL)
				}
				return store, nil
			}
		}
		return nil, fmt.Errorf("provider %q is not configured", provider)
	}
	for _, store := range s.stores {
		if store.SupportsURL(prURL) {
			return store, nil
		}
	}
	return nil, fmt.Errorf("no configured provider supports URL %q", prURL)
}

func buildStatusesSummary(statuses []scm.Status) scm.StatusesSummary {
	latest := map[string]scm.Status{}
	for _, status := range statuses {
		key := status.Key
		if key == "" {
			key = status.Name
		}
		if key == "" {
			key = status.URL
		}
		if prev, ok := latest[key]; !ok || status.UpdatedAt > prev.UpdatedAt {
			latest[key] = status
		}
	}
	summary := scm.StatusesSummary{Total: len(latest)}
	for _, status := range latest {
		switch normalizeStatus(status.State) {
		case "success":
			summary.Successful++
		case "failed":
			summary.Failed++
			summary.FailedStatuses = append(summary.FailedStatuses, status)
		case "in_progress":
			summary.InProgress++
		case "pending":
			summary.Pending++
		default:
			summary.Unknown++
		}
	}
	return summary
}

func normalizeStatus(state string) string {
	switch strings.ToLower(state) {
	case "successful", "success", "passed":
		return "success"
	case "failed", "failure", "error", "canceled", "cancelled", "skipped", "stopped":
		return "failed"
	case "running", "inprogress", "in_progress", "created", "manual":
		return "in_progress"
	case "pending", "waiting":
		return "pending"
	default:
		return "unknown"
	}
}

func normalizeBody(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}

func isDuplicate(existing []scm.Comment, file string, line int, normalizedBody string) bool {
	for _, comment := range existing {
		if comment.File == file && comment.Line == line && normalizeBody(comment.Body) == normalizedBody {
			return true
		}
	}
	return false
}
