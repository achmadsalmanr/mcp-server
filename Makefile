.PHONY: test build build-windows build-linux build-linux-arm64 build-macos build-macos-arm64 build-all clean clean-windows clean-unix

BINARY=mcp-server
HOST_OS=$(shell go env GOOS)
HOST_ARCH=$(shell go env GOARCH)
HOST_EXT=$(if $(filter windows,$(HOST_OS)),.exe,)
HOST_BINARY=$(BINARY)$(HOST_EXT)
CLEAN_ARTIFACTS=\
	$(BINARY) \
	$(BINARY).exe \
	$(BINARY)-windows-amd64.exe \
	$(BINARY)-linux-amd64 \
	$(BINARY)-linux-arm64 \
	$(BINARY)-darwin-amd64 \
	$(BINARY)-darwin-arm64

test:
	go test ./...

build:
	go build -o $(HOST_BINARY) ./cmd/mcp-server

build-windows: export GOOS=windows
build-windows: export GOARCH=amd64
build-windows:
	go build -o $(BINARY)-windows-amd64.exe ./cmd/mcp-server

build-linux: export GOOS=linux
build-linux: export GOARCH=amd64
build-linux:
	go build -o $(BINARY)-linux-amd64 ./cmd/mcp-server

build-linux-arm64: export GOOS=linux
build-linux-arm64: export GOARCH=arm64
build-linux-arm64:
	go build -o $(BINARY)-linux-arm64 ./cmd/mcp-server

build-macos: export GOOS=darwin
build-macos: export GOARCH=amd64
build-macos:
	go build -o $(BINARY)-darwin-amd64 ./cmd/mcp-server

build-macos-arm64: export GOOS=darwin
build-macos-arm64: export GOARCH=arm64
build-macos-arm64:
	go build -o $(BINARY)-darwin-arm64 ./cmd/mcp-server

build-all: build-windows build-linux build-linux-arm64 build-macos build-macos-arm64

ifeq ($(HOST_OS),windows)
clean: clean-windows
else
clean: clean-unix
endif

clean-windows:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/clean.ps1

clean-unix:
	rm -f $(CLEAN_ARTIFACTS)
