#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Local CI runner - equivalent to GitHub Actions CI pipeline.
    Runs ALL checks inside Docker. NO Go installation required locally.

.PARAMETER Only
    Run only this stage: gofmt | vet | lint | tests | build | e2e | docker

.PARAMETER Skip
    Comma-separated stages to skip. E.g. -Skip docker,e2e

.PARAMETER Fix
    Auto-fix gofmt formatting issues

.PARAMETER NoPull
    Skip Docker image pulls (faster when already cached)

.EXAMPLE
    .\ci-local.ps1
    .\ci-local.ps1 -Only tests
    .\ci-local.ps1 -Skip docker
    .\ci-local.ps1 -Fix -Only gofmt
#>

param(
    [string]$Only  = "",
    [string]$Skip  = "",
    [switch]$Fix,
    [switch]$NoPull,
    [switch]$Help
)

if ($Help) { Get-Help $MyInvocation.MyCommand.Path -Detailed; exit 0 }

$ErrorActionPreference = "Stop"
$GO_IMAGE  = "golang:1.24-alpine"
$PY_IMAGE  = "python:3.11-slim"
$ROOT      = $PSScriptRoot
$script:FAILURES = @()
$SkipList  = if ($Skip) { $Skip.ToLower() -split "," | ForEach-Object { $_.Trim() } } else { @() }

function Write-Step { param($m) Write-Host "`n=== $m ===" -ForegroundColor Cyan }
function Write-Ok   { param($m) Write-Host "  [OK]   $m"  -ForegroundColor Green }
function Write-Fail { param($m) Write-Host "  [FAIL] $m"  -ForegroundColor Red }
function Write-Info { param($m) Write-Host "  ...    $m"  -ForegroundColor Gray }
function Write-Warn { param($m) Write-Host "  [SKIP] $m"  -ForegroundColor Yellow }

function Should-Run([string]$stage) {
    if ($Only -and $Only.ToLower() -ne $stage) { return $false }
    if ($SkipList -contains $stage)             { return $false }
    return $true
}

function Run-Stage([string]$Name, [scriptblock]$Body) {
    if (-not (Should-Run $Name)) { Write-Warn "Skipping: $Name"; return }
    Write-Step $Name
    try   { & $Body; Write-Ok "$Name PASSED" }
    catch { Write-Fail "$Name FAILED: $_"; $script:FAILURES += $Name }
}

function Invoke-DockerGo([string]$ScriptFile) {
    docker run --rm `
        --workdir /workspace `
        -v "${ROOT}:/workspace" `
        -v "skypanel-gomodcache:/root/go/pkg/mod" `
        -v "skypanel-gocache:/root/.cache/go-build" `
        $GO_IMAGE sh /workspace/ci-scripts/$ScriptFile
    if ($LASTEXITCODE -ne 0) { throw "exit code $LASTEXITCODE" }
}

function Invoke-DockerPy([string]$ScriptFile) {
    docker run --rm `
        --workdir /workspace `
        -v "${ROOT}:/workspace" `
        $PY_IMAGE sh /workspace/ci-scripts/$ScriptFile
    if ($LASTEXITCODE -ne 0) { throw "exit code $LASTEXITCODE" }
}

# Pre-flight check
Write-Host ""
Write-Host "============================================" -ForegroundColor White
Write-Host "  Aether Panel - Local CI Runner"           -ForegroundColor White
Write-Host "============================================" -ForegroundColor White

$null = docker info 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Fail "Docker is not running. Please start Docker Desktop."
    exit 1
}
Write-Ok "Docker is running"

if (-not $NoPull) {
    Write-Info "Pulling Docker images (use -NoPull to skip)..."
    docker pull $GO_IMAGE 2>&1 | Select-String "Status|Pull complete|already" | ForEach-Object { Write-Info $_.Line }
    docker pull $PY_IMAGE 2>&1 | Select-String "Status|Pull complete|already" | ForEach-Object { Write-Info $_.Line }
}

# Frontend dist stub required for go:embed
$stubFile = Join-Path $ROOT "client\frontend\dist\index.html"
if (-not (Test-Path $stubFile)) {
    New-Item -ItemType Directory -Path (Split-Path $stubFile) -Force | Out-Null
    Set-Content -Path $stubFile "<!-- CI stub -->"
    Write-Info "Created frontend dist stub"
}

# STAGE 1: gofmt
Run-Stage "gofmt" {
    if ($Fix) { Invoke-DockerGo "01_gofmt_fix.sh"; return }
    Invoke-DockerGo "01_gofmt.sh"
}

# STAGE 2: go vet
Run-Stage "vet"    { Invoke-DockerGo "02_vet.sh"   }

# STAGE 3: golangci-lint
Run-Stage "lint"   { Invoke-DockerGo "03_lint.sh"  }

# STAGE 4: go tests
Run-Stage "tests"  { Invoke-DockerGo "04_tests.sh" }

# STAGE 5: build binary
Run-Stage "build"  { Invoke-DockerGo "05_build.sh" }

# STAGE 6: Python E2E
Run-Stage "e2e"    { Invoke-DockerPy "06_e2e.sh"   }

# STAGE 7: Docker build
Run-Stage "docker" {
    Write-Info "Building Docker image (local only, no push)..."
    docker build `
        --build-arg version=local-dev `
        --build-arg sha=local `
        -t skypanel-local:ci-test `
        -f (Join-Path $ROOT "Dockerfile") `
        $ROOT
    if ($LASTEXITCODE -ne 0) { throw "Docker build failed" }
    Write-Ok "Image: skypanel-local:ci-test"
    docker images skypanel-local:ci-test
}

# Summary
Write-Host ""
Write-Host "============================================" -ForegroundColor White
if ($script:FAILURES.Count -eq 0) {
    Write-Host "  ALL STAGES PASSED - safe to commit and push!" -ForegroundColor Green
    Write-Host ""
    Write-Host "  git add ."               -ForegroundColor DarkGray
    Write-Host "  git commit -m msg"       -ForegroundColor DarkGray
    Write-Host "  git push origin dev2.0"  -ForegroundColor DarkGray
    exit 0
} else {
    Write-Host "  FAILED: $($script:FAILURES -join ", ")" -ForegroundColor Red
    Write-Host ""
    Write-Host "  gofmt fix  -> .\ci-local.ps1 -Fix -Only gofmt" -ForegroundColor DarkGray
    Write-Host "  One stage  -> .\ci-local.ps1 -Only tests"      -ForegroundColor DarkGray
    Write-Host "  Skip stage -> .\ci-local.ps1 -Skip docker"     -ForegroundColor DarkGray
    exit 1
}
