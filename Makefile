# Flare 常用命令
.PHONY: build test coverage fmt vet

build:
	go build ./...

test:
	go test ./... -count=1

# 生成覆盖率报告（coverage.out、coverage.html）
coverage:
	go test ./... -count=1 -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

fmt:
	@gofmt -s -l . | grep -q . && (gofmt -s -d .; exit 1) || true

vet:
	go vet ./...
