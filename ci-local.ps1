#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Local CI runner - equivalent to GitHub Actions CI pipeline.
    Runs ALL checks inside Docker. NO Go installation required locally.

.PARAMETER Only
    Run only this stage: gofmt | vet | lint | staticcheck | gosec | trivy | tests | frontend | build | e2e | docker

.PARAMETER Skip
    Comma-separated stages to skip. E.g. -Skip docker,e2e,trivy

.PARAMETER Fix
    Auto-fix gofmt formatting issues

.PARAMETER NoPull
    Skip Docker image pulls (faster when already cached)

.EXAMPLE
    .\ci-local.ps1
    .\ci-local.ps1 -Only tests
    .\ci-local.ps1 -Skip docker,trivy
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
$GO_IMAGE    = "golang:1.25-alpine"
$PY_IMAGE    = "python:3.11-slim"
$NODE_IMAGE  = "node:22-alpine"
$TRIVY_IMAGE = "aquasec/trivy:latest"
$ROOT        = $PSScriptRoot
$script:FAILURES = @()
$SkipList  = if ($Skip) { $Skip.ToLower() -split "," | ForEach-Object { $_.Trim() } } else { @() }

function Write-Step { param($m) Write-Host "`n=== $m ===" -ForegroundColor Cyan }
function Write-Ok   { param($m) Write-Host "  [OK]   $m"  -ForegroundColor Green }
function Write-Fail { param($m) Write-Host "  [FAIL] $m"  -ForegroundColor Red }
function Write-Info { param($m) Write-Host "  ...    $m"  -ForegroundColor Gray }
function Write-Warn { param($m) Write-Host "  [SKIP] $m"  -ForegroundColor Yellow }

function Should-Run([string]$stage) {
    if ($Only) {
        $onlyList = $Only.ToLower() -split "," | ForEach-Object { $_.Trim() }
        if ($onlyList -notcontains $stage) { return $false }
    }
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
        -v "skypanel-gomodcache:/go/pkg/mod" `
        -v "skypanel-gocache:/root/.cache/go-build" `
        -e GOPROXY=https://proxy.golang.org,direct `
        --dns 8.8.8.8 `
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

function Invoke-DockerNode([string]$ScriptFile) {
    docker run --rm `
        --workdir /workspace `
        -v "${ROOT}:/workspace" `
        -v "skypanel-nodemodules:/workspace/client/node_modules" `
        -v "skypanel-nodemodules-frontend:/workspace/client/frontend/node_modules" `
        --dns 8.8.8.8 `
        $NODE_IMAGE sh /workspace/ci-scripts/$ScriptFile
    if ($LASTEXITCODE -ne 0) { throw "exit code $LASTEXITCODE" }
}

function Invoke-DockerTrivy([string]$ScanArgs) {
    $skipDirs = 'node_modules,.git,.cache,client/node_modules,client/frontend/node_modules,bin'
    $dockerArgs = @(
        'run', '--rm',
        '-v', "${ROOT}:/workspace",
        $TRIVY_IMAGE, 'fs',
        '--exit-code', '1',
        '--format', 'table',
        '--skip-dirs', $skipDirs
    ) + (-split $ScanArgs) + @('/workspace')
    & docker $dockerArgs
    if ($LASTEXITCODE -ne 0) { throw "exit code $LASTEXITCODE" }
}

# Trivy DB cache volume to reuse across runs
$TRIVY_CACHE_VOLUME = "trivy-db-cache"
$null = docker volume create $TRIVY_CACHE_VOLUME 2>&1

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
    docker pull $NODE_IMAGE 2>&1 | Select-String "Status|Pull complete|already" | ForEach-Object { Write-Info $_.Line }
    docker pull $TRIVY_IMAGE 2>&1 | Select-String "Status|Pull complete|already" | ForEach-Object { Write-Info $_.Line }
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

# STAGE 4: staticcheck
Run-Stage "staticcheck" { Invoke-DockerGo "07_staticcheck.sh" }

# STAGE 5: gosec
Run-Stage "gosec" { Invoke-DockerGo "08_gosec.sh" }

# STAGE 6: Trivy (vulnerability, secret, config scans)
Run-Stage "trivy" {
    Write-Info "Trivy - Vulnerability scan (CRITICAL+HIGH)..."
    Invoke-DockerTrivy "--ignore-unfixed --severity CRITICAL,HIGH --pkg-types os,library"
    Write-Info "Trivy - Secret scan..."
    Invoke-DockerTrivy "--scanners secret"
    Write-Info "Trivy - Config scan..."
    Invoke-DockerTrivy "--scanners config"
}

# STAGE 7: go tests
Run-Stage "tests"  { Invoke-DockerGo "04_tests.sh" }

# STAGE 8: Frontend quality (lint, typecheck, build)
Run-Stage "frontend" { Invoke-DockerNode "09_frontend.sh" }

# STAGE 9: build binary
Run-Stage "build"  { Invoke-DockerGo "05_build.sh" }

# STAGE 10: Python E2E
Run-Stage "e2e"    { Invoke-DockerPy "06_e2e.sh"   }

# STAGE 11: Docker build
Run-Stage "docker" {
    Write-Info "Cleaning reparse points in node_modules (Windows Docker workaround)..."
    Get-ChildItem -Path (Join-Path $ROOT "client") -Recurse -Directory -Force -ErrorAction SilentlyContinue `
        | Where-Object { $_.Attributes -band [System.IO.FileAttributes]::ReparsePoint } `
        | ForEach-Object {
            Write-Info "  Removing reparse point: $($_.FullName)"
            Remove-Item -LiteralPath $_.FullName -Recurse -Force -ErrorAction SilentlyContinue
        }
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
    Write-Host "  gofmt fix     -> .\ci-local.ps1 -Fix -Only gofmt"      -ForegroundColor DarkGray
    Write-Host "  One stage     -> .\ci-local.ps1 -Only tests"           -ForegroundColor DarkGray
    Write-Host "  Skip stage    -> .\ci-local.ps1 -Skip docker,trivy"    -ForegroundColor DarkGray
    Write-Host "  Skip frontend -> .\ci-local.ps1 -Skip frontend"        -ForegroundColor DarkGray
    exit 1
}
