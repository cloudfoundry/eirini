.PHONY: all build ci clean dependencies format ginkgo test

ifeq ($(GOOS),windows)
DEST = build/credhub.exe
else
DEST = build/credhub
endif

ifndef VERSION
VERSION = dev
endif

GOFLAGS := -o $(DEST) -ldflags "-X github.com/cloudfoundry-incubator/credhub-cli/version.Version=${VERSION}"

all: test clean build

clean:
	rm -rf build

format:
	go fmt .

ginkgo:
	ginkgo -r -randomizeSuites -randomizeAllSpecs -race -p 2>&1

test: format ginkgo

ci: ginkgo

build:
	mkdir -p build
	go build $(GOFLAGS)
