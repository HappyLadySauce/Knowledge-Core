#Requires -Version 5.1
<#
.SYNOPSIS
  Regenerate Swagger / OpenAPI artifacts for gin-swagger (api/swagger/docs).

.DESCRIPTION
  Runs swag from the repository root regardless of the caller's current directory.
  在仓库根目录执行 swag，与调用时的当前目录无关。

.EXAMPLE
  .\scripts\api\swag.ps1
#>

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location -LiteralPath $RepoRoot

Write-Host "swag: repo root = $RepoRoot"
Write-Host 'swag: output dir = api/swagger/docs'

# -g root.go with -d ./cmd matches swag's expected layout (avoids cmd/cmd/root.go resolution bugs).
# 使用 -g root.go 与 -d ./cmd 符合 swag 预期布局（避免出现 cmd/cmd/root.go 解析错误）。
$swag = @(
    'run', 'github.com/swaggo/swag/cmd/swag@v1.8.12', 'init',
    '-g', 'root.go',
    '-d', './cmd',
    '-o', 'api/swagger/docs',
    '--parseInternal'
)
& go @swag
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

Write-Host 'swag: done.'
