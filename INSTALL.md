# Installing mcp-server

`mcp-server` generates MCP stdio config for any compatible client. It does not need Docker.

## Build

```bash
go test ./...
go build -o mcp-server.exe ./cmd/mcp-server
```

On macOS/Linux you can build without `.exe`:

```bash
go build -o mcp-server ./cmd/mcp-server
```

With `make`, the default build target follows your current OS:

```bash
make build
```

Output names:

| Command | Output |
|---|---|
| `make build` on Windows | `mcp-server.exe` |
| `make build` on macOS/Linux | `mcp-server` |
| `make build-windows` | `mcp-server-windows-amd64.exe` |
| `make build-linux` | `mcp-server-linux-amd64` |
| `make build-linux-arm64` | `mcp-server-linux-arm64` |
| `make build-macos` | `mcp-server-darwin-amd64` |
| `make build-macos-arm64` | `mcp-server-darwin-arm64` |
| `make build-all` | all cross-build artifacts |

## Clean Build Artifacts

On Windows PowerShell:

```powershell
.\scripts\clean.ps1
```

Preview what would be removed:

```powershell
.\scripts\clean.ps1 -WhatIf
```

On macOS/Linux:

```bash
make clean
```

## Where To Put Env Vars

Put tokens in your own user environment, not in this repository.

The server reads env vars at runtime:

```powershell
$env:GITHUB_TOKEN="github_pat_xxx"
$env:GITLAB_TOKEN="glpat_xxx"
$env:BITBUCKET_WORKSPACE="your-workspace"
$env:BITBUCKET_EMAIL="you@example.com"
$env:BITBUCKET_API_TOKEN="your-bitbucket-token"
$env:TESTRAIL_BASE_URL="https://yourcompany.testrail.io"
$env:TESTRAIL_EMAIL="you@example.com"
$env:TESTRAIL_API_KEY="your-testrail-api-key"
```

Those PowerShell commands only apply to the current terminal. For persistent Windows user env vars:

```powershell
setx GITHUB_TOKEN "github_pat_xxx"
setx GITLAB_TOKEN "glpat_xxx"
setx BITBUCKET_WORKSPACE "your-workspace"
setx BITBUCKET_EMAIL "you@example.com"
setx BITBUCKET_API_TOKEN "your-bitbucket-token"
setx TESTRAIL_BASE_URL "https://yourcompany.testrail.io"
setx TESTRAIL_EMAIL "you@example.com"
setx TESTRAIL_API_KEY "your-testrail-api-key"
```

Restart your MCP client after using `setx`.

On macOS/Linux:

```bash
export GITHUB_TOKEN="github_pat_xxx"
export GITLAB_TOKEN="glpat_xxx"
export BITBUCKET_WORKSPACE="your-workspace"
export BITBUCKET_EMAIL="you@example.com"
export BITBUCKET_API_TOKEN="your-bitbucket-token"
export TESTRAIL_BASE_URL="https://yourcompany.testrail.io"
export TESTRAIL_EMAIL="you@example.com"
export TESTRAIL_API_KEY="your-testrail-api-key"
```

## Safe MCP Config

Default generated config references env vars instead of writing token literals:

```bash
./mcp-server.exe install --target generic --print-config
```

Example output:

```json
{
  "mcpServers": {
    "multi-git-testrail": {
      "command": "D:\\Project\\mcp-server\\mcp-server.exe",
      "env": {
        "GITHUB_TOKEN": "${GITHUB_TOKEN}",
        "GITLAB_TOKEN": "${GITLAB_TOKEN}",
        "BITBUCKET_API_TOKEN": "${BITBUCKET_API_TOKEN}",
        "TESTRAIL_API_KEY": "${TESTRAIL_API_KEY}"
      }
    }
  }
}
```

If your MCP client does not expand `${ENV_VAR}` references, put the env vars in that client's user-level secret/env settings, or paste literal values only in a private user config file that is never committed.

Only use this when you intentionally want plaintext values in the generated config:

```bash
./mcp-server.exe install --target generic --print-config --include-secrets
```

## Security Rules

- Do not commit tokens, `.env` files, or MCP config files containing plaintext credentials.
- Prefer user-level MCP config over project-level config for secrets.
- Share this repo and binary safely; each person should provide their own env vars.
- Use least-privilege tokens. Read-only tokens are enough for read tools; write tools need PR/MR/comment permissions.

## Provider Env Vars

| Provider | Required for private/write access |
|---|---|
| Bitbucket | `BITBUCKET_WORKSPACE`, `BITBUCKET_EMAIL`, `BITBUCKET_API_TOKEN` |
| GitHub | `GITHUB_TOKEN` |
| GitHub Enterprise | `GITHUB_TOKEN`, `GITHUB_WEB_BASE_URL`, `GITHUB_API_BASE_URL`, optional `GITHUB_API_VERSION` |
| GitLab | `GITLAB_TOKEN`, optional `GITLAB_BASE_URL` for self-managed GitLab |
| TestRail | `TESTRAIL_BASE_URL`, `TESTRAIL_EMAIL`, `TESTRAIL_API_KEY` |

GitHub and GitLab public repositories can be read without a token. Private repositories and all write tools require a token.

## GitHub Token Permissions

For a fine-grained GitHub personal access token, select the repository you want to access and use:

| Feature | GitHub repository permission |
|---|---|
| Read PR metadata, commits, diff, and review comments | `Pull requests: Read` |
| Read PR issue-level comments | `Pull requests: Read` or `Issues: Read` |
| Read commit/build statuses | `Commit statuses: Read` |
| Post inline PR review comments or request changes/approve | `Pull requests: Write` |
| Post PR summary comments | `Pull requests: Write` or `Issues: Write` |

`get_pr_context` can still read the PR when `Commit statuses: Read` is missing. It returns an empty status summary and includes a warning.

## Diagnostics

```bash
./mcp-server.exe doctor
./mcp-server.exe doctor --json
```

Inside an MCP chat, call:

```json
{ "name": "provider_status", "arguments": {} }
```

## Quick Workflow

1. Build `mcp-server.exe`.
2. Set your own env vars in your OS or MCP client.
3. Run `./mcp-server.exe install --target generic --print-config`.
4. Paste the config into your MCP client.
5. Restart the MCP client.
6. Run `provider_status` to confirm what is configured.
