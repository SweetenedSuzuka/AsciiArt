# AsciiArt 构建流程。
#
#   make all      生成 dist/asciiart.exe 与 dist/asciiart-gui.exe
#   make package  额外打包 zip 与校验值
#   make help     列出全部目标

SHELL := /bin/sh

BIN_CLI := asciiart.exe
BIN_GUI := asciiart-gui.exe
DIST    := dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

GOFLAGS := -trimpath
LDFLAGS := -s -w \
	-X asciiart/internal/cli.Version=$(VERSION) \
	-X main.Version=$(VERSION)

.PHONY: all cli gui test vet fmt fmt-check clean package release run-cli run-gui help

all: cli gui

## cli: 构建单文件命令行工具
cli:
	@mkdir -p $(DIST)
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/$(BIN_CLI) ./cmd/asciiart
	@echo "built $(DIST)/$(BIN_CLI)"

## gui: 构建图形界面版本（运行时不显示控制台窗口）
gui:
	@mkdir -p $(DIST)
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS) -H windowsgui" -o $(DIST)/$(BIN_GUI) ./cmd/asciiart-gui
	@echo "built $(DIST)/$(BIN_GUI)"

## test: 运行单元测试与解码器模糊测试的初始输入用例
test:
	go test ./...

## vet: 运行标准静态检查
vet:
	# 关闭 unsafeptr 检查：Win32 绑定中消息参数以 uintptr 类型传入，需要重新解释为指针。
	# 静态分析器无法把这种用法与指针运算错误区分开。
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

## package: 构建两个二进制文件，打包为 zip 并生成校验值
package: all
	go run ./tools/package -version $(VERSION) -dist $(DIST) -out $(DIST)
	@ls -la $(DIST)

## release: 依次执行格式检查、静态检查与测试，然后构建并打包
release: fmt-check vet test package

## run-cli: 用内置示例图片快速运行命令行版本
run-cli:
	go run ./cmd/asciiart demo.png -m ascii -w 72

## run-gui: 用内置示例图片启动图形界面版本
run-gui:
	go run ./cmd/asciiart-gui demo.png

## help: 列出全部目标
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
