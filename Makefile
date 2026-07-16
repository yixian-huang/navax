SHELL := /bin/sh

APP := navax
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || printf dev)
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || printf unknown)
BUILT_AT ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
DEPLOYMENT ?= binary
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.builtAt=$(BUILT_AT) -X main.deployment=$(DEPLOYMENT)

.PHONY: frontend check test test-contract e2e e2e-install embed build clean

web/node_modules/.install-stamp: web/package.json web/package-lock.json
	cd web && npm ci
	touch $@

frontend: web/node_modules/.install-stamp
	cd web && npm run build

check: web/node_modules/.install-stamp
	cd web && npm run type-check && npm run lint
	@files="$$(gofmt -l $$(find cmd internal migrations tests -type f -name '*.go'))"; \
		test -z "$$files" || { printf '以下 Go 文件需要 gofmt：\n%s\n' "$$files"; exit 1; }
	go vet ./...

test:
	go test ./...

# 契约测试：启动真实二进制并对每对请求/响应做 OpenAPI 校验（内部会自行构建）。
test-contract:
	go test ./tests/contract/ -count=1

tests/e2e/node_modules/.install-stamp: tests/e2e/package.json
	cd tests/e2e && npm install
	cd tests/e2e && npx playwright install --with-deps chromium
	touch $@

e2e-install: tests/e2e/node_modules/.install-stamp

# 端到端测试：针对内嵌前端的真实二进制运行游客/用户/管理员关键路径。
e2e: build tests/e2e/node_modules/.install-stamp
	cd tests/e2e && npx playwright test

embed: frontend
	rm -rf internal/webui/dist
	mkdir -p internal/webui/dist
	cp -R web/out/. internal/webui/dist/
	printf '%s\n' 'Run the frontend build before a production Go build.' > internal/webui/dist/placeholder.txt

build: embed
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="$(LDFLAGS)" -o bin/$(APP) ./cmd/navax

clean:
	rm -rf bin web/out internal/webui/dist tests/e2e/test-results tests/e2e/playwright-report
	mkdir -p internal/webui/dist
	printf '%s\n' 'Run the frontend build before a production Go build.' > internal/webui/dist/placeholder.txt
