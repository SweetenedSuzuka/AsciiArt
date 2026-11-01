# AsciiArt 构建流程。
#
#   make all      生成 dist/asciiart.exe 与 dist/asciiart-gui.exe
#   make package  额外生成 zip 压缩包与校验值
#   make help     列出全部目标

SHELL := /bin/sh

BIN_CLI := asciiart.exe
BIN_GUI := asciiart-gui.exe
DIST    := dist

# 标签名如果有美元符号、反引号和斜杠直接读取会造成错误，比如美元符号与反引号即使放在双引号里也会被 shell 展开、斜杠会改变输出路径。
# 因此把字母、数字、点、加号、下划线和连字符以外的字符一律替换成连字符，只过滤 git 给出的标签名，通过环境变量或命令行传入的 VERSION 由调用方保证只含这些字符。
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | tr -c 'A-Za-z0-9._+\n-' '-')
ifeq ($(strip $(VERSION)),)
VERSION := dev
endif

GOFLAGS := -trimpath
LDFLAGS := -s -w \
	-X asciiart/internal/cli.Version=$(VERSION) \
	-X main.Version=$(VERSION)

.PHONY: all cli gui test vet fmt fmt-check clean package release run-cli run-gui help

## all: 构建命令行版本与图形界面版本
all: cli gui

## cli: 构建命令行版本
cli:
	@mkdir -p $(DIST)
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/$(BIN_CLI) ./cmd/asciiart
	@echo "built $(DIST)/$(BIN_CLI)"

## gui: 构建图形界面版本（运行时不显示控制台窗口）
gui:
	@mkdir -p $(DIST)
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS) -H windowsgui" -o $(DIST)/$(BIN_GUI) ./cmd/asciiart-gui
	@echo "built $(DIST)/$(BIN_GUI)"

## test: 运行全部测试，模糊测试目标以 f.Add 添加的固定输入运行
test:
	go test ./...

## vet: 运行标准静态检查
# 关闭 unsafeptr 检查：Win32 绑定中消息参数以 uintptr 类型传入，需要重新解释为指针，静态分析器无法区分这种情况与指针运算错误。
vet:
	go vet -unsafeptr=false ./...

## fmt: 格式化全部源文件
fmt:
	gofmt -w .

## fmt-check: 有文件未通过 gofmt 时报错退出
fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "以下文件未通过 gofmt："; echo "$$out"; exit 1; fi

## clean: 删除构建产物
clean:
	rm -rf $(DIST)

## package: 构建命令行版本与图形界面版本，打包为 zip 并生成校验值
package: all
	go run ./tools/package -version "$(VERSION)" -dist "$(DIST)" -out "$(DIST)"
	@ls -la $(DIST)

## release: 依次执行格式检查、静态检查与测试，然后构建并打包
release: fmt-check vet test package

## run-cli: 使用示例图片 demo.png 运行命令行版本
run-cli:
	go run ./cmd/asciiart demo.png -m ascii -w 72

## run-gui: 使用示例图片 demo.png 启动图形界面版本
run-gui:
	go run ./cmd/asciiart-gui demo.png

## help: 列出全部目标
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
