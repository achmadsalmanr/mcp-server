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
$env:FIGMA_ACCESS_TOKEN="figd_xxx"
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
setx FIGMA_ACCESS_TOKEN "figd_xxx"
```

Restart your MCP client after using `setx`.

On macOS/Linux, `export` makes a value available to child processes started from
that shell:

```bash
export GITHUB_TOKEN="github_pat_xxx"
export GITLAB_TOKEN="glpat_xxx"
export BITBUCKET_WORKSPACE="your-workspace"
export BITBUCKET_EMAIL="you@example.com"
export BITBUCKET_API_TOKEN="your-bitbucket-token"
export TESTRAIL_BASE_URL="https://yourcompany.testrail.io"
export TESTRAIL_EMAIL="you@example.com"
export TESTRAIL_API_KEY="your-testrail-api-key"
export FIGMA_ACCESS_TOKEN="figd_xxx"
```

To load these values in future shells, put the required `export` commands in a
shell startup file:

| Shell | Common startup file |
|---|---|
| zsh | `~/.zprofile` for login shells, or `~/.zshrc` for interactive shells |
| bash | `~/.bash_profile` or `~/.profile` for login shells, or `~/.bashrc` for interactive shells |

After editing the file, start a new shell or source the file explicitly. The MCP
client only passes variables it had when it started, so restart the client after
changing its environment.

Applications launched from the macOS Dock or a Linux application menu do not
reliably read zsh/bash startup files. In that case, use the wrapper below instead
of putting token values in the IDE's MCP config.

## Portable Shared MCP Config

The most portable config to share contains no `env` block and no credentials:

```json
{
  "mcpServers": {
    "multi-mcp": {
      "command": "/absolute/path/to/mcp-server"
    }
  }
}
```

Every user replaces the command with the absolute path to their binary. With no
`env` block, the server inherits variables from the MCP client process. This works
when the client was started from an environment containing the required variables.

## Generated Config And Interpolation

Default generated config references env vars instead of writing token literals:

```bash
./mcp-server.exe install --target generic --print-config
```

The current generator always includes an `env` block. Remove that block manually
when using the command-only inheritance setup above.

Example output:

```json
{
  "mcpServers": {
    "multi-mcp": {
      "command": "D:\\Project\\mcp-server\\mcp-server.exe",
      "env": {
        "GITHUB_TOKEN": "${GITHUB_TOKEN}",
        "GITLAB_TOKEN": "${GITLAB_TOKEN}",
        "BITBUCKET_API_TOKEN": "${BITBUCKET_API_TOKEN}",
        "TESTRAIL_API_KEY": "${TESTRAIL_API_KEY}",
        "FIGMA_ACCESS_TOKEN": "${FIGMA_ACCESS_TOKEN}"
      }
    }
  }
}
```

`${ENV_VAR}` expansion is performed by the MCP client, not by `mcp-server`. Some
clients do not support that syntax and pass the reference literally. An explicit
literal reference also overrides a real inherited variable; `mcp-server` treats
unresolved `${...}` values as unset.

Use generated placeholder config only when your MCP client documents support for
that interpolation syntax. Otherwise choose one of these approaches:

- Remove the `env` block and let the server inherit exported variables.
- Use the MCP client's supported user-level secret or environment forwarding feature.
- Use the private wrapper below for a GUI-launched client.
- Put literal values in a private user config only as a last resort; never commit it.

Use `--include-secrets` only when you intentionally want plaintext values in the
generated config:

```bash
./mcp-server.exe install --target generic --print-config --include-secrets
```

## macOS/Linux Wrapper For GUI Clients

A wrapper gives a GUI-launched MCP client a reliable environment without storing
tokens in the IDE config. Create `~/.config/multi-mcp/env` with only the variables
needed by this server:

```sh
mkdir -p "$HOME/.config/multi-mcp" "$HOME/.local/bin"
```

```sh
FIGMA_ACCESS_TOKEN='figd_xxx'
# Add other provider variables here when needed.
```

Create `~/.local/bin/multi-mcp`:

```sh
#!/bin/sh
set -a
. "$HOME/.config/multi-mcp/env"
set +a

exec "$HOME/.local/bin/mcp-server"
```

Keep the binary at the path used by the wrapper, make the wrapper executable, and
restrict access to the environment file:

```sh
chmod 700 "$HOME/.local/bin/multi-mcp"
chmod 600 "$HOME/.config/multi-mcp/env"
```

Point the MCP config at the wrapper using an absolute path; MCP clients may not
expand `~` or `$HOME` in a command:

```json
{
  "mcpServers": {
    "multi-mcp": {
      "command": "/Users/your-name/.local/bin/multi-mcp"
    }
  }
}
```

On Linux the equivalent path is commonly
`/home/your-name/.local/bin/multi-mcp`. The wrapper and any shell startup files it
loads must not print to stdout because stdout carries the MCP protocol.

The private environment file is still plaintext storage. For stronger protection,
have the wrapper retrieve tokens from macOS Keychain, Linux Secret Service, or a
password-manager CLI and export them only for the server process.

## Security Rules

- Do not commit tokens, `.env` files, or MCP config files containing plaintext credentials.
- Prefer user-level MCP config over project-level config for secrets.
- Share this repo and binary safely; each person should provide their own env vars.
- Treat shell profiles and private environment files as plaintext secret storage; restrict their permissions.
- Use least-privilege tokens. Read-only tokens are enough for read tools; write tools need PR/MR/comment permissions.

## Provider Env Vars

| Provider | Required for private/write access |
|---|---|
| Bitbucket | `BITBUCKET_WORKSPACE`, `BITBUCKET_EMAIL`, `BITBUCKET_API_TOKEN` |
| GitHub | `GITHUB_TOKEN` |
| GitHub Enterprise | `GITHUB_TOKEN`, `GITHUB_WEB_BASE_URL`, `GITHUB_API_BASE_URL`, optional `GITHUB_API_VERSION` |
| GitLab | `GITLAB_TOKEN`, optional `GITLAB_BASE_URL` for self-managed GitLab |
| TestRail | `TESTRAIL_BASE_URL`, `TESTRAIL_EMAIL`, `TESTRAIL_API_KEY` |
| Figma | `FIGMA_ACCESS_TOKEN`, optional `FIGMA_API_BASE_URL` for Figma Gov |

GitHub and GitLab public repositories can be read without a token. Private repositories and all write tools require a token.

Figma API requests require a Figma personal access token or plan access token. Use `file_content:read` for file/node/image tools, `file_comments:read` for reading comments, and `file_comments:write` for posting comments.

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

If `doctor` reports a provider as configured but `provider_status` reports its
variables as missing, the MCP child process did not receive the same environment.
Check for an unexpanded `${ENV_VAR}` entry, remove unsupported `env` mappings, and
restart or reconnect the MCP client.

## Quick Workflow

1. Build the binary for your OS.
2. Set your own environment variables or create a private wrapper.
3. Add a command-only config to the MCP client, or use generated placeholders only when the client supports them.
4. Restart or reconnect the MCP client.
5. Run `provider_status` to confirm what the MCP process received.
