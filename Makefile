# Local builds. `make build` writes ./bin/pbrain with the same tags as the release
# archive for this machine (see .goreleaser.yaml); `make build TAGS=` builds CPU-only.

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

ifeq ($(GOOS)/$(GOARCH),linux/amd64)
TAGS ?= cuda,vulkan,eg2_combined,eg2_fp16
else ifeq ($(GOOS)/$(GOARCH),linux/arm64)
TAGS ?= cuda,eg2_combined,eg2_fp16
else ifeq ($(GOOS)/$(GOARCH),darwin/arm64)
TAGS ?= metal
else
TAGS ?=
endif

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/pkieltyka/pbrain/internal/cli.version=$(VERSION)

.PHONY: build clean

build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o bin/pbrain ./cmd/pbrain

clean:
	rm -rf bin
