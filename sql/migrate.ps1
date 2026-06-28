# Knowledge-Core SQLite migration entrypoint for Windows PowerShell.
# Knowledge-Core SQLite 数据库迁移入口（Windows PowerShell）。

param(
    [string]$Db = '',
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$MigrationsRoot = Join-Path $RepoRoot 'sql\migrations'

if (-not $Db) {
    $Db = $env:KNOWLEDGE_CORE_SQLITE_PATH
}
if (-not $Db) {
    $Db = '.knowledge-core/index.db'
}

$goArgs = @(
    'run', './sql/migrate/main.go',
    '-db', $Db,
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
