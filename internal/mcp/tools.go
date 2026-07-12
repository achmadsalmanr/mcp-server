package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/achmadsalmanr/mcp-server/internal/figma"
	"github.com/achmadsalmanr/mcp-server/internal/review"
	"github.com/achmadsalmanr/mcp-server/internal/testrail"
)

func (s *Server) registerTools() {
	s.tools = []Tool{
		s.providerStatusTool(),
		s.figmaGetFileTool(),
		s.figmaGetFileNodesTool(),
		s.figmaGetImagesTool(),
		s.figmaGetImageFillsTool(),
		s.figmaGetFileCommentsTool(),
		s.figmaPostFileCommentTool(),
		s.getPRContextTool(),
		s.getPRDiffTool(),
		s.listPRCommentsTool(),
		s.listPRTasksTool(),
		s.listPRStatusesTool(),
		s.draftReviewCommentsTool(),
		s.postReviewCommentsTool(),
		s.approvePRTool(),
		s.requestChangesPRTool(),
		s.resolveTaskTool(),
		s.reopenTaskTool(),
		s.resolveCommentThreadTool(),
		s.reopenCommentThreadTool(),
		s.testrailGetCaseTool(),
		s.testrailListCasesTool(),
		s.testrailGetRunTool(),
		s.testrailListTestsTool(),
		s.testrailAddResultTool(),
		s.testrailAddResultForCaseTool(),
	}
}

func (s *Server) providerStatusTool() Tool {
	return Tool{
		Name:        "provider_status",
		Description: "Show configured providers and missing environment variables.",
		InputSchema: objectSchema(map[string]any{}),
		Annotations: readOnly(),
		Handler: func(context.Context, json.RawMessage) (any, error) {
			return map[string]any{
				"providers":          s.statuses,
				"warnings":           s.warnings,
				"registered_scm":     s.review.Providers(),
				"testrail_available": s.testrail != nil && s.testrail.Configured(),
				"figma_available":    s.figma != nil && s.figma.Configured(),
			}, nil
		},
	}
}

func (s *Server) figmaGetFileTool() Tool {
	return Tool{
		Name:        "figma_get_file",
		Description: "Fetch a Figma file document JSON by file key, branch key, or full Figma URL.",
		InputSchema: objectSchema(map[string]any{
			"file_key":    stringProp("Figma file key, branch key, or full Figma file URL."),
			"version":     stringProp("Optional version ID. Defaults to the current version."),
			"ids":         stringArrayProp("Optional node IDs. If set, Figma returns those subtrees and ancestors."),
			"depth":       intProp("Optional positive traversal depth."),
			"geometry":    enumProp("Optional geometry export mode.", "paths"),
			"plugin_data": stringArrayProp("Optional plugin IDs and/or shared to include plugin data."),
			"branch_data": boolProp("Return branch metadata for the requested file."),
		}, "file_key"),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[figma.GetFileInput](args)
			if err != nil {
				return nil, err
			}
			return s.requireFigma().GetFile(ctx, input)
		},
	}
}

func (s *Server) figmaGetFileNodesTool() Tool {
	return Tool{
		Name:        "figma_get_file_nodes",
		Description: "Fetch selected Figma nodes by node IDs or by a Figma URL containing node-id.",
		InputSchema: objectSchema(map[string]any{
			"file_key":    stringProp("Figma file key, branch key, or full Figma node URL."),
			"ids":         stringArrayProp("Node IDs to retrieve. Optional when file_key is a Figma URL with node-id."),
			"version":     stringProp("Optional version ID. Defaults to the current version."),
			"depth":       intProp("Optional positive traversal depth under the selected nodes."),
			"geometry":    enumProp("Optional geometry export mode.", "paths"),
			"plugin_data": stringArrayProp("Optional plugin IDs and/or shared to include plugin data."),
		}, "file_key"),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[figma.GetFileNodesInput](args)
			if err != nil {
				return nil, err
			}
			return s.requireFigma().GetFileNodes(ctx, input)
		},
	}
}

func (s *Server) figmaGetImagesTool() Tool {
	return Tool{
		Name:        "figma_get_images",
		Description: "Render selected Figma nodes and return temporary image URLs.",
		InputSchema: objectSchema(map[string]any{
			"file_key":            stringProp("Figma file key, branch key, or full Figma node URL."),
			"ids":                 stringArrayProp("Node IDs to render. Optional when file_key is a Figma URL with node-id."),
			"scale":               numberProp("Optional image scale from 0.01 to 4."),
			"format":              enumProp("Optional image format.", "jpg", "png", "svg", "pdf"),
			"svg_outline_text":    boolProp("For SVG, render text as vector outlines."),
			"svg_include_id":      boolProp("For SVG, include layer names as id attributes."),
			"svg_include_node_id": boolProp("For SVG, include node IDs as data-node-id attributes."),
			"svg_simplify_stroke": boolProp("For SVG, simplify strokes where possible."),
			"contents_only":       boolProp("Exclude content overlapping the selected node."),
			"use_absolute_bounds": boolProp("Use full node dimensions instead of cropped bounds."),
			"version":             stringProp("Optional version ID. Defaults to the current version."),
		}, "file_key"),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[figma.GetImagesInput](args)
			if err != nil {
				return nil, err
			}
			return s.requireFigma().GetImages(ctx, input)
		},
	}
}

func (s *Server) figmaGetImageFillsTool() Tool {
	return Tool{
		Name:        "figma_get_image_fills",
		Description: "Return temporary download URLs for image fills used in a Figma file.",
		InputSchema: objectSchema(map[string]any{
			"file_key": stringProp("Figma file key, branch key, or full Figma file URL."),
		}, "file_key"),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[struct {
				FileKey string `json:"file_key"`
			}](args)
			if err != nil {
				return nil, err
			}
			return s.requireFigma().GetImageFills(ctx, input.FileKey)
		},
	}
}

func (s *Server) figmaGetFileCommentsTool() Tool {
	return Tool{
		Name:        "figma_get_file_comments",
		Description: "List comments left on a Figma file.",
		InputSchema: objectSchema(map[string]any{
			"file_key": stringProp("Figma file key, branch key, or full Figma file URL."),
			"as_md":    boolProp("Return comment text as Markdown when Figma can provide it."),
		}, "file_key"),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[figma.GetCommentsInput](args)
			if err != nil {
				return nil, err
			}
			return s.requireFigma().GetComments(ctx, input)
		},
	}
}

func (s *Server) figmaPostFileCommentTool() Tool {
	return Tool{
		Name:        "figma_post_file_comment",
		Description: "Post a new Figma file comment or reply to an existing root comment.",
		InputSchema: objectSchema(map[string]any{
			"file_key":    stringProp("Figma file key, branch key, or full Figma file URL."),
			"message":     stringProp("Text contents of the comment to post."),
			"comment_id":  stringProp("Optional root comment ID to reply to."),
			"client_meta": map[string]any{"type": "object", "description": "Optional Figma comment position metadata."},
		}, "file_key", "message"),
		Annotations: writeTool(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[figma.PostCommentInput](args)
			if err != nil {
				return nil, err
			}
			return s.requireFigma().PostComment(ctx, input)
		},
	}
}

func (s *Server) getPRContextTool() Tool {
	return Tool{
		Name:        "get_pr_context",
		Description: "Return PR/MR metadata, commits, comments, tasks where supported, statuses summary, and the first structured diff page for Bitbucket, GitHub, or GitLab.",
		InputSchema: objectSchema(map[string]any{
			"provider":       enumProp("Optional provider override.", "bitbucket", "github", "gitlab"),
			"pr_url":         stringProp("Full Bitbucket PR, GitHub PR, or GitLab MR URL."),
			"max_diff_lines": intProp("Maximum diff lines to return. Defaults to 500."),
			"max_files":      intProp("Maximum diff files to return on the first page."),
			"diff_cursor":    stringProp("Cursor returned by a previous diff page."),
		}, "pr_url"),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[getPRContextInput](args)
			if err != nil {
				return nil, err
			}
			if input.PRURL == "" {
				return nil, fmt.Errorf("pr_url is required")
			}
			return s.review.GetPRContext(ctx, review.GetPRContextInput{
				Provider:     input.Provider,
				PRURL:        input.PRURL,
				MaxDiffLines: input.MaxDiffLines,
				MaxFiles:     input.MaxFiles,
				DiffCursor:   input.DiffCursor,
			})
		},
	}
}

func (s *Server) getPRDiffTool() Tool {
	return Tool{
		Name:        "get_pr_diff",
		Description: "Return a paginated structured diff for a Bitbucket PR, GitHub PR, or GitLab MR. Use next_cursor for subsequent pages.",
		InputSchema: objectSchema(map[string]any{
			"provider":  enumProp("Optional provider override.", "bitbucket", "github", "gitlab"),
			"pr_url":    stringProp("Full PR/MR URL."),
			"file":      stringProp("Optional exact file path filter."),
			"cursor":    stringProp("Cursor returned by the previous page."),
			"max_lines": intProp("Maximum diff lines to return. Defaults to 500."),
			"max_files": intProp("Maximum changed files to return."),
		}, "pr_url"),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[getPRDiffInput](args)
			if err != nil {
				return nil, err
			}
			if input.PRURL == "" {
				return nil, fmt.Errorf("pr_url is required")
			}
			return s.review.GetPRDiff(ctx, review.GetPRDiffInput{
				Provider: input.Provider,
				PRURL:    input.PRURL,
				File:     input.File,
				Cursor:   input.Cursor,
				MaxLines: input.MaxLines,
				MaxFiles: input.MaxFiles,
			})
		},
	}
}

func (s *Server) listPRCommentsTool() Tool {
	return Tool{
		Name:        "list_pr_comments",
		Description: "List comments on a Bitbucket PR, GitHub PR, or GitLab MR.",
		InputSchema: prURLSchema(),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[prURLInput](args)
			if err != nil {
				return nil, err
			}
			ctxOut, err := s.review.GetPRContext(ctx, review.GetPRContextInput{Provider: input.Provider, PRURL: input.PRURL, MaxDiffLines: 1})
			if err != nil {
				return nil, err
			}
			return map[string]any{"comments": ctxOut.Comments}, nil
		},
	}
}

func (s *Server) listPRTasksTool() Tool {
	return Tool{
		Name:        "list_pr_tasks",
		Description: "List PR tasks where the provider supports tasks. Bitbucket supports this; GitHub and GitLab return an empty list.",
		InputSchema: prURLSchema(),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[prURLInput](args)
			if err != nil {
				return nil, err
			}
			ctxOut, err := s.review.GetPRContext(ctx, review.GetPRContextInput{Provider: input.Provider, PRURL: input.PRURL, MaxDiffLines: 1})
			if err != nil {
				return nil, err
			}
			return map[string]any{"tasks": ctxOut.Tasks}, nil
		},
	}
}

func (s *Server) listPRStatusesTool() Tool {
	return Tool{
		Name:        "list_pr_statuses",
		Description: "Return a status summary for the PR/MR source commit.",
		InputSchema: prURLSchema(),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[prURLInput](args)
			if err != nil {
				return nil, err
			}
			ctxOut, err := s.review.GetPRContext(ctx, review.GetPRContextInput{Provider: input.Provider, PRURL: input.PRURL, MaxDiffLines: 1})
			if err != nil {
				return nil, err
			}
			return map[string]any{"summary": ctxOut.StatusesSummary}, nil
		},
	}
}

func (s *Server) draftReviewCommentsTool() Tool {
	return Tool{
		Name:        "draft_review_comments",
		Description: "Validate review findings against the current diff and existing comments. Produces a stateless draft; no writes are performed.",
		InputSchema: objectSchema(map[string]any{
			"provider": enumProp("Optional provider override.", "bitbucket", "github", "gitlab"),
			"pr_url":   stringProp("Full PR/MR URL."),
			"findings": map[string]any{
				"type": "array",
				"items": objectSchema(map[string]any{
					"file":        stringProp("File path in the diff."),
					"new_line_no": intProp("New-file line number from get_pr_diff/get_pr_context."),
					"body":        stringProp("Review comment body."),
					"severity":    enumProp("Optional severity.", "HIGH", "MEDIUM", "LOW"),
					"create_task": boolProp("Create a task if supported by the provider."),
				}, "file", "new_line_no", "body"),
			},
			"summary_body":    stringProp("Optional PR-level summary comment."),
			"summary_verdict": enumProp("Optional summary verdict.", "PASS", "COMMENT", "BLOCK"),
		}, "pr_url", "findings"),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[draftReviewInput](args)
			if err != nil {
				return nil, err
			}
			return s.review.DraftReviewComments(ctx, review.DraftReviewInput{
				Provider:       input.Provider,
				PRURL:          input.PRURL,
				Findings:       input.Findings,
				SummaryBody:    input.SummaryBody,
				SummaryVerdict: input.SummaryVerdict,
			})
		},
	}
}

func (s *Server) postReviewCommentsTool() Tool {
	return Tool{
		Name:        "post_review_comments",
		Description: "Post an approved draft from draft_review_comments after checking the source commit has not changed.",
		InputSchema: objectSchema(map[string]any{"draft": map[string]any{"type": "object"}}, "draft"),
		Annotations: writeTool(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[postReviewInput](args)
			if err != nil {
				return nil, err
			}
			return s.review.PostReviewComments(ctx, input.Draft)
		},
	}
}

func (s *Server) approvePRTool() Tool {
	return Tool{
		Name:        "approve_pr",
		Description: "Approve a Bitbucket PR, GitHub PR, or GitLab MR after checking expected_source_commit_hash.",
		InputSchema: stateChangeSchema(),
		Annotations: writeTool(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[stateChangeInput](args)
			if err != nil {
				return nil, err
			}
			return s.review.ApprovePR(ctx, review.ApprovePRInput{
				Provider:                 input.Provider,
				PRURL:                    input.PRURL,
				ExpectedSourceCommitHash: input.ExpectedSourceCommitHash,
			})
		},
	}
}

func (s *Server) requestChangesPRTool() Tool {
	return Tool{
		Name:        "request_changes_pr",
		Description: "Request changes on a Bitbucket or GitHub PR after checking expected_source_commit_hash. GitLab does not support this operation in this server.",
		InputSchema: stateChangeSchema(),
		Annotations: writeTool(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[stateChangeInput](args)
			if err != nil {
				return nil, err
			}
			return s.review.RequestChangesPR(ctx, review.RequestChangesInput{
				Provider:                 input.Provider,
				PRURL:                    input.PRURL,
				ExpectedSourceCommitHash: input.ExpectedSourceCommitHash,
			})
		},
	}
}

func (s *Server) resolveTaskTool() Tool {
	return taskTool("resolve_task", "Resolve a PR task where supported.", func(ctx context.Context, svc *review.Service, input taskInput) (any, error) {
		return svc.ResolveTask(ctx, review.ResolveTaskInput{Provider: input.Provider, PRURL: input.PRURL, TaskID: input.TaskID})
	}, s.review)
}

func (s *Server) reopenTaskTool() Tool {
	return taskTool("reopen_task", "Reopen a PR task where supported.", func(ctx context.Context, svc *review.Service, input taskInput) (any, error) {
		return svc.ReopenTask(ctx, review.ReopenTaskInput{Provider: input.Provider, PRURL: input.PRURL, TaskID: input.TaskID})
	}, s.review)
}

func (s *Server) resolveCommentThreadTool() Tool {
	return commentThreadTool("resolve_comment_thread", "Resolve a PR/MR comment thread where supported.", func(ctx context.Context, svc *review.Service, input commentThreadInput) error {
		return svc.ResolveCommentThread(ctx, review.ResolveCommentThreadInput{Provider: input.Provider, PRURL: input.PRURL, CommentID: input.CommentID})
	}, s.review)
}

func (s *Server) reopenCommentThreadTool() Tool {
	return commentThreadTool("reopen_comment_thread", "Reopen a PR/MR comment thread where supported.", func(ctx context.Context, svc *review.Service, input commentThreadInput) error {
		return svc.ReopenCommentThread(ctx, review.ReopenCommentThreadInput{Provider: input.Provider, PRURL: input.PRURL, CommentID: input.CommentID})
	}, s.review)
}

func (s *Server) testrailGetCaseTool() Tool {
	return Tool{
		Name:        "testrail_get_case",
		Description: "Fetch one TestRail case by case ID.",
		InputSchema: objectSchema(map[string]any{"case_id": intProp("TestRail case ID.")}, "case_id"),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[struct {
				CaseID int `json:"case_id"`
			}](args)
			if err != nil {
				return nil, err
			}
			return s.requireTestRail().GetCase(ctx, input.CaseID)
		},
	}
}

func (s *Server) testrailListCasesTool() Tool {
	return Tool{
		Name:        "testrail_list_cases",
		Description: "List TestRail cases for a project, optionally filtered by suite or section.",
		InputSchema: objectSchema(map[string]any{
			"project_id": intProp("TestRail project ID."),
			"suite_id":   intProp("Optional suite ID."),
			"section_id": intProp("Optional section ID."),
			"limit":      intProp("Optional page size."),
			"offset":     intProp("Optional offset."),
		}, "project_id"),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[testrail.ListCasesInput](args)
			if err != nil {
				return nil, err
			}
			cases, err := s.requireTestRail().ListCases(ctx, input)
			if err != nil {
				return nil, err
			}
			return map[string]any{"cases": cases}, nil
		},
	}
}

func (s *Server) testrailGetRunTool() Tool {
	return Tool{
		Name:        "testrail_get_run",
		Description: "Fetch one TestRail run by run ID.",
		InputSchema: objectSchema(map[string]any{"run_id": intProp("TestRail run ID.")}, "run_id"),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[struct {
				RunID int `json:"run_id"`
			}](args)
			if err != nil {
				return nil, err
			}
			return s.requireTestRail().GetRun(ctx, input.RunID)
		},
	}
}

func (s *Server) testrailListTestsTool() Tool {
	return Tool{
		Name:        "testrail_list_tests",
		Description: "List tests in a TestRail run.",
		InputSchema: objectSchema(map[string]any{
			"run_id": intProp("TestRail run ID."),
			"limit":  intProp("Optional page size."),
			"offset": intProp("Optional offset."),
		}, "run_id"),
		Annotations: readOnly(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[testrail.ListTestsInput](args)
			if err != nil {
				return nil, err
			}
			tests, err := s.requireTestRail().ListTests(ctx, input)
			if err != nil {
				return nil, err
			}
			return map[string]any{"tests": tests}, nil
		},
	}
}

func (s *Server) testrailAddResultTool() Tool {
	return Tool{
		Name:        "testrail_add_result",
		Description: "Add a result to a TestRail test.",
		InputSchema: resultSchema(map[string]any{"test_id": intProp("TestRail test ID.")}, "test_id"),
		Annotations: writeTool(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[testrail.AddResultInput](args)
			if err != nil {
				return nil, err
			}
			return s.requireTestRail().AddResult(ctx, input)
		},
	}
}

func (s *Server) testrailAddResultForCaseTool() Tool {
	return Tool{
		Name:        "testrail_add_result_for_case",
		Description: "Add a result to a TestRail case inside a run.",
		InputSchema: resultSchema(map[string]any{
			"run_id":  intProp("TestRail run ID."),
			"case_id": intProp("TestRail case ID."),
		}, "run_id", "case_id"),
		Annotations: writeTool(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[testrail.AddResultForCaseInput](args)
			if err != nil {
				return nil, err
			}
			return s.requireTestRail().AddResultForCase(ctx, input)
		},
	}
}

func (s *Server) requireTestRail() *testrail.Client {
	if s.testrail == nil || !s.testrail.Configured() {
		return testrail.NewClient("", "", "")
	}
	return s.testrail
}

func (s *Server) requireFigma() *figma.Client {
	if s.figma == nil || !s.figma.Configured() {
		return figma.NewClient("", "")
	}
	return s.figma
}

func prURLSchema() map[string]any {
	return objectSchema(map[string]any{
		"provider": enumProp("Optional provider override.", "bitbucket", "github", "gitlab"),
		"pr_url":   stringProp("Full PR/MR URL."),
	}, "pr_url")
}

func stateChangeSchema() map[string]any {
	return objectSchema(map[string]any{
		"provider":                    enumProp("Optional provider override.", "bitbucket", "github", "gitlab"),
		"pr_url":                      stringProp("Full PR/MR URL."),
		"expected_source_commit_hash": stringProp("Source commit hash from get_pr_context."),
	}, "pr_url", "expected_source_commit_hash")
}

func resultSchema(extra map[string]any, required ...string) map[string]any {
	properties := map[string]any{
		"status_id":     intProp("TestRail status ID."),
		"comment":       stringProp("Optional result comment."),
		"version":       stringProp("Optional tested version."),
		"elapsed":       stringProp("Optional elapsed time, e.g. 1m or 30s."),
		"defects":       stringProp("Optional comma-separated defect references."),
		"assignedto_id": intProp("Optional assignee user ID."),
	}
	for k, v := range extra {
		properties[k] = v
	}
	required = append(required, "status_id")
	return objectSchema(properties, required...)
}

type getPRContextInput struct {
	Provider     string `json:"provider,omitempty"`
	PRURL        string `json:"pr_url"`
	MaxDiffLines int    `json:"max_diff_lines,omitempty"`
	MaxFiles     int    `json:"max_files,omitempty"`
	DiffCursor   string `json:"diff_cursor,omitempty"`
}

type getPRDiffInput struct {
	Provider string `json:"provider,omitempty"`
	PRURL    string `json:"pr_url"`
	File     string `json:"file,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
	MaxLines int    `json:"max_lines,omitempty"`
	MaxFiles int    `json:"max_files,omitempty"`
}

type prURLInput struct {
	Provider string `json:"provider,omitempty"`
	PRURL    string `json:"pr_url"`
}

type draftReviewInput struct {
	Provider       string                 `json:"provider,omitempty"`
	PRURL          string                 `json:"pr_url"`
	Findings       []review.ReviewFinding `json:"findings"`
	SummaryBody    string                 `json:"summary_body,omitempty"`
	SummaryVerdict string                 `json:"summary_verdict,omitempty"`
}

type postReviewInput struct {
	Draft review.ReviewDraft `json:"draft"`
}

type stateChangeInput struct {
	Provider                 string `json:"provider,omitempty"`
	PRURL                    string `json:"pr_url"`
	ExpectedSourceCommitHash string `json:"expected_source_commit_hash"`
}

type taskInput struct {
	Provider string `json:"provider,omitempty"`
	PRURL    string `json:"pr_url"`
	TaskID   string `json:"task_id"`
}

type commentThreadInput struct {
	Provider  string `json:"provider,omitempty"`
	PRURL     string `json:"pr_url"`
	CommentID string `json:"comment_id"`
}

func taskTool(name, description string, run func(context.Context, *review.Service, taskInput) (any, error), svc *review.Service) Tool {
	return Tool{
		Name:        name,
		Description: description,
		InputSchema: objectSchema(map[string]any{
			"provider": enumProp("Optional provider override.", "bitbucket", "github", "gitlab"),
			"pr_url":   stringProp("Full PR/MR URL."),
			"task_id":  stringProp("Provider task ID."),
		}, "pr_url", "task_id"),
		Annotations: writeTool(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[taskInput](args)
			if err != nil {
				return nil, err
			}
			return run(ctx, svc, input)
		},
	}
}

func commentThreadTool(name, description string, run func(context.Context, *review.Service, commentThreadInput) error, svc *review.Service) Tool {
	return Tool{
		Name:        name,
		Description: description,
		InputSchema: objectSchema(map[string]any{
			"provider":   enumProp("Optional provider override.", "bitbucket", "github", "gitlab"),
			"pr_url":     stringProp("Full PR/MR URL."),
			"comment_id": stringProp("Provider comment or thread ID. For GitLab, pass the id returned by list_pr_comments."),
		}, "pr_url", "comment_id"),
		Annotations: writeTool(),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			input, err := decodeArgs[commentThreadInput](args)
			if err != nil {
				return nil, err
			}
			if err := run(ctx, svc, input); err != nil {
				return nil, err
			}
			return map[string]bool{"success": true}, nil
		},
	}
}
