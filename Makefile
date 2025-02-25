# 主要用于简化go项目的构建和代码检查过程
# build 目标：用于编译项目的 main.go 文件，并在编译时注入版本、构建时间、Git 提交信息等。同时，支持通过设置 gorace 变量为 1 来启用竞态检测
# lint 目标：用于运行 golangci-lint 工具对项目代码进行静态分析，确保代码符合一定的规范和质量标准
SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")# 使用Makefile命令在当前目录下查找所有类型为普通文件且以.go结尾的文件，排除vendor目录下的文件

VERSION := v1.0.0

# 通过 git rev-parse --abbrev-ref HEAD 命令获取当前的 Git 分支名称
CHANNEL := $(shell git rev-parse --abbrev-ref HEAD)
# 将当前分支名称和 Git 提交的短哈希值（取前 7 位）拼接起来，作为构建版本标识
CHANNEL_BUILD = $(CHANNEL)-$(shell git rev-parse --short=7 HEAD)
# 定义项目的导入路径
project=github.com/dszqbsm/crawler

# 定义链接器标志变量，用于在编译时将一些信息注入到可执行文件中
LDFLAGS = -X "github.com/dszqbsm/crawler/version.BuildTS=$(shell date -u '+%Y-%m-%d %I:%M:%S')"# 构建时间
LDFLAGS += -X "github.com/dszqbsm/crawler/version.GitHash=$(shell git rev-parse HEAD)"# Git 提交哈希值
LDFLAGS += -X "github.com/dszqbsm/crawler/version.GitBranch=$(shell git rev-parse --abbrev-ref HEAD)"# Git 分支名称
LDFLAGS += -X "github.com/dszqbsm/crawler/version.Version=${VERSION}"# 版本号

# 竞态检测标志
ifeq ($(gorace), 1)
# 如果设置了 gorace 变量为 1，则启用竞态检测
	BUILD_FLAGS=-race
endif

# build目标，用于编译项目的 main.go 文件，通过将之前定义的链接器标志传递给 go build 命令实现在编译时注入版本和构建信息
# 由make build命令调用
build:
	go build -ldflags '$(LDFLAGS)' $(BUILD_FLAGS) main.go

# lint目标，用于运行 golangci-lint 工具对项目代码进行静态分析，确保代码符合一定的规范和质量标准
# 由make lint命令调用
lint:
	golangci-lint run ./...