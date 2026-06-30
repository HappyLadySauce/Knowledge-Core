# Knowledge-Core PostgreSQL migration entrypoint for Windows PowerShell.
# Knowledge-Core PostgreSQL 数据库迁移入口（Windows PowerShell）。

param(
    [string]$DatabaseUrl = '',
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$MigrationsRoot = Join-Path $RepoRoot 'sql\migrations'

if (-not $DatabaseUrl) {
    $DatabaseUrl = $env:KNOWLEDGE_CORE_DATABASE_URL
}
if (-not $DatabaseUrl) {
    $DatabaseUrl = 'postgres://knowledge_core:knowledge_core@localhost:5432/knowledge_core?sslmode=disable'
}

$goArgs = @(
    'run', './sql/migrate/main.go',
    '-database-url', $DatabaseUrl,
    '-dir', $MigrationsRoot
)
if ($Force) {
    $goArgs += '-force'
}

Push-Location $RepoRoot
try {
    & go @goArgs
} finally {
    Pop-Location
}
