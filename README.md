# mcp-server

A Docker-free MCP server for working with Bitbucket pull requests, GitHub pull requests, GitLab merge requests, and TestRail.

The server runs over MCP stdio using newline-delimited JSON-RPC. Build one local binary, point your MCP client at it, and configure only the providers you need with environment variables.

## Build

```bash
go test ./...
go build -o mcp-server ./cmd/mcp-server
```

On Windows this produces `mcp-server.exe`.

## Configure

Full install and token-safety notes are in [INSTALL.md](INSTALL.md).

Print generic MCP config:

```bash
./mcp-server install --target generic --print-config
```

Run a local configuration check:

```bash
./mcp-server doctor
./mcp-server doctor --json
```

Use these environment variables as needed:

| Provider | Environment |
|---|---|
| Bitbucket | `BITBUCKET_WORKSPACE`, `BITBUCKET_EMAIL`, `BITBUCKET_API_TOKEN` |
| GitHub | `GITHUB_TOKEN` |
| GitHub Enterprise | `GITHUB_TOKEN`, `GITHUB_WEB_BASE_URL`, `GITHUB_API_BASE_URL`, optional `GITHUB_API_VERSION` |
| GitLab | `GITLAB_TOKEN`, optional `GITLAB_BASE_URL` for self-managed GitLab |
| TestRail | `TESTRAIL_BASE_URL`, `TESTRAIL_EMAIL`, `TESTRAIL_API_KEY` |

GitHub and GitLab can read public repositories without a token. Private repositories and write tools require a token.

Generated MCP config uses `${ENV_VAR}` references by default so token literals do not get written into committed files. Use `--include-secrets` only for private user config files.

## SCM Tools

- `provider_status`
- `get_pr_context`
- `get_pr_diff`
- `list_pr_comments`
- `list_pr_tasks`
- `list_pr_statuses`
- `draft_review_comments`
- `post_review_comments`
- `approve_pr`
- `request_changes_pr`
- `resolve_task`
- `reopen_task`
- `resolve_comment_thread`
- `reopen_comment_thread`

`get_pr_context`, `get_pr_diff`, and review tools accept Bitbucket PR URLs, GitHub PR URLs, or GitLab MR URLs. The provider is detected from the URL, or you can pass `provider`.

Write tools use stale-source guards. Fetch `source_commit_hash` with `get_pr_context`, then pass it as `expected_source_commit_hash` for approval/request-changes operations or use the draft returned by `draft_review_comments` for posting review comments.

## TestRail Tools

- `testrail_get_case`
- `testrail_list_cases`
- `testrail_get_run`
- `testrail_list_tests`
- `testrail_add_result`
- `testrail_add_result_for_case`

## Provider Notes

Bitbucket supports PR comments, tasks, statuses, approvals, request changes, and task/thread state changes.

GitHub supports PR comments, issue-level summary comments, statuses, approvals, and request changes. GitHub REST does not expose simple thread resolve/reopen in this server.

GitLab supports MR comments through discussions, statuses, approvals, and discussion resolve/reopen. GitLab request-changes is not implemented as a generic write operation here.

TestRail support covers common case/run/test/result operations.
