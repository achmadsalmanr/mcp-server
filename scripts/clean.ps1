[CmdletBinding(SupportsShouldProcess = $true)]
param()

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot '..')
$Artifacts = @(
    'mcp-server',
    'mcp-server.exe',
    'mcp-server-windows-amd64.exe',
    'mcp-server-linux-amd64',
    'mcp-server-linux-arm64',
    'mcp-server-darwin-amd64',
    'mcp-server-darwin-arm64'
)

foreach ($Artifact in $Artifacts) {
    $Path = Join-Path $RepoRoot $Artifact
    if (Test-Path -LiteralPath $Path) {
        if ($PSCmdlet.ShouldProcess($Path, 'Remove build artifact')) {
            Remove-Item -LiteralPath $Path -Force
        }
    }
}
