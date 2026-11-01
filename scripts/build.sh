#!/usr/bin/env bash
#
# AsciiArt 构建入口脚本。
#
#   scripts/build.sh              # 构建命令行版本与图形界面版本，输出到 dist/ 目录
#   scripts/build.sh --package    # 额外生成发布用的 zip 压缩包与校验值
#   scripts/build.sh --skip-tests # 跳过测试步骤
#   scripts/build.sh --help       # 显示这段说明
#   VERSION=1.2.0 scripts/build.sh
#
# 可在 Git Bash、MSYS2、WSL 等带有 bash 的环境中运行，不依赖 make。

set -euo pipefail

cd "$(dirname "$0")/.."

DIST="dist"
VERSION="${VERSION:-}"
DO_PACKAGE=0
SKIP_TESTS=0

for arg in "$@"; do
	case "$arg" in
	--package) DO_PACKAGE=1 ;;
	--skip-tests) SKIP_TESTS=1 ;;
	-h | --help)
		sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'
		exit 0
		;;
	*)
		echo "未知参数：$arg" >&2
		exit 2
		;;
	esac
done

if [ -z "$VERSION" ]; then
	if VERSION=$(git describe --tags --always --dirty 2>/dev/null); then
		:
	else
		VERSION="dev"
	fi
fi

case "$VERSION" in
	*[!0-9A-Za-z._+-]*)
		echo "错误：版本号只允许字母、数字、点、加号、下划线和连字符，目前值为：$VERSION" >&2
		exit 2
		;;
esac

echo "==> AsciiArt $VERSION"

if ! command -v go >/dev/null 2>&1; then
	echo "错误：未找到 go，请先安装 Go 1.27 或更高版本" >&2
	exit 1
fi

# 项目没有 cgo 代码，把 CGO_ENABLED 设为 0，产物是单个静态可执行文件。
export CGO_ENABLED=0

echo "==> 检查格式"
unformatted=$(gofmt -l . || true)
if [ -n "$unformatted" ]; then
	echo "以下文件未通过 gofmt，请先运行 gofmt -w ." >&2
	echo "$unformatted" >&2
	exit 1
fi

echo "==> go vet"
go vet -unsafeptr=false ./...

if [ "$SKIP_TESTS" -eq 0 ]; then
	echo "==> go test"
	go test ./...
fi

mkdir -p "$DIST"

LDFLAGS="-s -w -X asciiart/internal/cli.Version=$VERSION -X main.Version=$VERSION"

echo "==> 构建命令行版本"
go build -trimpath -ldflags "$LDFLAGS" -o "$DIST/asciiart.exe" ./cmd/asciiart

echo "==> 构建图形界面版本"
go build -trimpath -ldflags "$LDFLAGS -H windowsgui" -o "$DIST/asciiart-gui.exe" ./cmd/asciiart-gui

echo
echo "==> 产物"
ls -lh "$DIST"/*.exe | awk '{printf "    %-28s %s\n", $9, $5}'

if [ "$DO_PACKAGE" -eq 1 ]; then
	echo
	echo "==> 打包"
	go run ./tools/package -version "$VERSION" -dist "$DIST" -out "$DIST"
fi

echo
echo "完成。"
