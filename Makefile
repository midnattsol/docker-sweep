.PHONY: build install uninstall clean test

VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)
BINARY := docker-sweep
PLUGIN_DIR := $(HOME)/.docker/cli-plugins

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

install: build
	@mkdir -p $(PLUGIN_DIR)
	cp $(BINARY) $(PLUGIN_DIR)/$(BINARY)
	@echo "Installed to $(PLUGIN_DIR)/$(BINARY)"
	@echo "Run: docker sweep"

uninstall:
	rm -f $(PLUGIN_DIR)/$(BINARY)
	@echo "Uninstalled $(BINARY)"

clean:
	rm -f $(BINARY)
	go clean

test:
	go test -v ./...

# Build for all platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-arm64 .

# Verify plugin metadata
verify:
	@./$(BINARY) docker-cli-plugin-metadata | jq .
