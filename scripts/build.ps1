# AsciiArt 构建入口脚本（PowerShell 版本）。
#
#   .\scripts\build.ps1                 # 构建命令行版本与图形界面版本，输出到 dist\ 目录
#   .\scripts\build.ps1 -Package        # 额外生成发布用的 zip 压缩包与校验值
#   .\scripts\build.ps1 -SkipTests
#   $env:VERSION = "1.2.0"; .\scripts\build.ps1

[CmdletBinding()]
param(
    [switch]$Package,
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

$dist = "dist"
$version = $env:VERSION
if (-not $version) {
    try { $version = (git describe --tags --always --dirty 2>$null) } catch { $version = $null }
    if (-not $version) { $version = "dev" }
}

Write-Host "==> AsciiArt $version"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "未找到 go，请先安装 Go 1.21 或更高版本"
}

# 项目没有 cgo 代码，产物为单个静态可执行文件。
$env:CGO_ENABLED = "0"

Write-Host "==> 检查格式"
$unformatted = & gofmt -l .
if ($unformatted) {
    Write-Error "以下文件未通过 gofmt，请先运行 gofmt -w .`n$unformatted"
}

Write-Host "==> go vet"
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw "go vet 失败" }

if (-not $SkipTests) {
    Write-Host "==> go test"
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw "测试失败" }
}

New-Item -ItemType Directory -Force -Path $dist | Out-Null

$ldflags = "-s -w -X asciiart/internal/cli.Version=$version -X main.Version=$version"

Write-Host "==> 构建命令行版本"
& go build -trimpath -ldflags $ldflags -o "$dist/asciiart.exe" ./cmd/asciiart
if ($LASTEXITCODE -ne 0) { throw "命令行版本构建失败" }

Write-Host "==> 构建图形界面版本"
& go build -trimpath -ldflags "$ldflags -H windowsgui" -o "$dist/asciiart-gui.exe" ./cmd/asciiart-gui
if ($LASTEXITCODE -ne 0) { throw "图形界面版本构建失败" }

Write-Host ""
Write-Host "==> 产物"
Get-ChildItem "$dist/*.exe" | ForEach-Object {
    "{0,-28} {1:N0} KB" -f $_.Name, ($_.Length / 1KB)
}

if ($Package) {
    Write-Host ""
    Write-Host "==> 打包"
    & go run ./tools/package -version $version -dist $dist -out $dist
    if ($LASTEXITCODE -ne 0) { throw "打包失败" }
}

Write-Host ""
Write-Host "完成。"
