ifdef CI_COMMIT_TAG
VERSION ?= ${CI_COMMIT_TAG}
else
# 从Git获取最新tag
VERSION ?= $(shell git describe --tags 2>/dev/null || echo "unknown")
endif

ifdef CI_COMMIT_SHORT_SHA
SHORT_COMMIT := ${CI_COMMIT_SHORT_SHA}
else
# 从Git获取当前提交ID，取前8位
SHORT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null | head -c8)
endif

ifdef CI_COMMIT_SHA
COMMIT := ${CI_COMMIT_SHA}
else
# 从Git获取当前提交ID，取前8位
COMMIT := $(shell git rev-parse HEAD 2>/dev/null)
endif

ifdef CI_COMMIT_BRANCH
GIT_BRANCH := ${CI_COMMIT_BRANCH}
else
# 从Git获取当前提交ID，取前8位
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "HEAD")
endif

ifdef CI_PROJECT_NAME
FILENAME ?= ${CI_PROJECT_NAME}
else
# 从go.mod里面提取项目名，作为编译文件名
FILENAME ?= $(shell head -1 go.mod | awk -F '[/ ]' '{print $$NF}' | cut -d. -f1)
endif

# RFC3339Nano格式的编译时间
MAKEDATE  := $(shell date '+%FT%T%:z')
# Go版本号
GO_VERSION  := $(shell go version | cut -d' ' -f3)
# 定义输出目录（加上 "_" 避免 go 扫描）
DISTDIR   ?= _dist
# 定义最终输出路径
BIN_FILE  := ${DISTDIR}/${FILENAME}
# 编译命令
BUILD_CMD := go build -trimpath -ldflags "-s -w -X main.version=${VERSION} -X main.buildDate=${MAKEDATE} -X main.commit=${COMMIT} -X main.gitBranch=${GIT_BRANCH} -X main.goVersion=${GO_VERSION}"
# 入口文件或文件夹
MAIN      := ./cmd/main
# 开启CGO
export CGO_ENABLED  := 1
export CGO_LDFLAGS  := -Wl,-rpath=$$ORIGIN/lib
export NODE_VERSION := 20

# 默认命令：根据编译当前系统当前架构的版本，不添加 系统-架构 后缀
.PHONY: dist
dist: libs
	$(BUILD_CMD) -o $(BIN_FILE) $(MAIN)

.PHONY: libs
libs:
	mkdir -p ${DISTDIR}/lib
	find /lib /lib64 /usr/lib /usr/lib64 -name 'libbrotli*.so.1' -print0 \
	| xargs -0 -I {} ln -svf {} ${DISTDIR}/lib/


.PHONY: package
package: dist
	tar -zch --remove-files --strip-components=1 \
	-C ${DISTDIR} \
	-f $(BIN_FILE)-${VERSION}.tar.gz \
	${FILENAME} lib

# 定义 make dep.* 为执行 go mod 的操作
.PHONY: deps.%
dep.%:
	go mod $(word 1,$(subst ., ,$*))

# 定义 make deps 为执行依赖检查、校验、下载
deps: dep.tidy dep.verify dep.download

.PHONY: web.install
web.install:
	@bash -l -c 'cd web; nvm exec $(NODE_VERSION) yarn install'

.PHONY: web.build
web.build:
	@bash -l -c 'cd web; nvm exec $(NODE_VERSION) yarn build'

.PHONY: web
web: web.install web.build

# 定义 make clean 为清理目录和编译缓存
clean:
	-rm -rf -- ${DISTDIR}
	go tool dist clean
# 	如果你想清理掉所有缓存
# 	go clean -cache

