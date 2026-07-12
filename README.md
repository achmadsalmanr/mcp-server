# mcp-server

A Docker-free MCP server for working with Bitbucket pull requests, GitHub pull requests, GitLab merge requests, TestRail, and Figma.

The server runs over MCP stdio using newline-delimited JSON-RPC. Build one local binary, point your MCP client at it, and configure only the providers you need with environment variables.

## Build

```bash
go test ./...
go build -o mcp-server ./cmd/mcp-server
```

On Windows this produces `mcp-server.exe`.

## Configure

Full install, environment inheritance, GUI client, and token-safety notes are in [INSTALL.md](INSTALL.md).

Print generic MCP config:

```bash
./mcp-server install --target generic --print-config
```

Run a local configuration check:

```bash
./mcp-server doctor
./mcp-server doctor --json
```

For a shareable MCP config that never contains credentials, configure only the
command and let the server inherit the MCP client's environment:

```json
{
  "mcpServers": {
    "multi-mcp": {
      "command": "/absolute/path/to/mcp-server"
    }
  }
}
```

Each user must set their own environment variables before the MCP client starts.
GUI-launched IDEs may not load shell profiles; use the private wrapper described
in [INSTALL.md](INSTALL.md#macoslinux-wrapper-for-gui-clients) when necessary.

Use these environment variables as needed:

| Provider | Environment |
|---|---|
| Bitbucket | `BITBUCKET_WORKSPACE`, `BITBUCKET_EMAIL`, `BITBUCKET_API_TOKEN` |
| GitHub | `GITHUB_TOKEN` |
| GitHub Enterprise | `GITHUB_TOKEN`, `GITHUB_WEB_BASE_URL`, `GITHUB_API_BASE_URL`, optional `GITHUB_API_VERSION` |
| GitLab | `GITLAB_TOKEN`, optional `GITLAB_BASE_URL` for self-managed GitLab |
| TestRail | `TESTRAIL_BASE_URL`, `TESTRAIL_EMAIL`, `TESTRAIL_API_KEY` |
| Figma | `FIGMA_ACCESS_TOKEN`, optional `FIGMA_API_BASE_URL` for Figma Gov |

GitHub and GitLab can read public repositories without a token. Private repositories and write tools require a token.

Generated MCP config uses `${ENV_VAR}` references by default so token literals do
not get written into committed files. Treat that output as a client-specific
template: some MCP clients do not expand `${ENV_VAR}`. An unresolved reference
can override the inherited value and is treated as unset by this server. If the
client does not explicitly support interpolation, remove the `env` block and use
environment inheritance or a private wrapper. Use `--include-secrets` only for
private user config files.

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

## Figma Tools

- `figma_get_file`
- `figma_get_file_nodes`
- `figma_get_images`
- `figma_get_image_fills`
- `figma_get_file_comments`
- `figma_post_file_comment`

Figma tools accept a file key, branch key, or full Figma file URL. Node tools can infer the node ID from URLs containing `node-id`.

Use a Figma personal or plan access token in `FIGMA_ACCESS_TOKEN`. File and image tools need `file_content:read`; comment reads need `file_comments:read`; posting comments needs `file_comments:write`.

## Provider Notes

Bitbucket supports PR comments, tasks, statuses, approvals, request changes, and task/thread state changes.

GitHub supports PR comments, issue-level summary comments, statuses, approvals, and request changes. GitHub REST does not expose simple thread resolve/reopen in this server.

GitLab supports MR comments through discussions, statuses, approvals, and discussion resolve/reopen. GitLab request-changes is not implemented as a generic write operation here.

TestRail support covers common case/run/test/result operations.
