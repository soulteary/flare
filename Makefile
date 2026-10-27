# Flare 常用命令
.PHONY: build test coverage fmt vet

build:
	go build ./...

test:
	go test ./... -count=1

# 与 CI 一致：全包语句覆盖率，启用 race 和 atomic 插桩
coverage:
	go test -race -count=1 -coverpkg=./... -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

fmt:
	@gofmt -s -l . | grep -q . && (gofmt -s -d .; exit 1) || true

vet:
	go vet ./...
