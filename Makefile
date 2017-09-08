.PHONY: all

SOURCES := $(shell find . 2>&1 | grep -E '.*\.(c|h|go)$$')
PREFIX := $(DESTDIR)/usr/local
BINDIR := $(PREFIX)/sbin
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_BRANCH_CLEAN := $(shell echo $(GIT_BRANCH) | sed -e "s/[^[:alnum:]]/-/g")
PROJECT := github.com/harche/runvm
BUILDTAGS := seccomp apparmor
COMMIT_NO := $(shell git rev-parse HEAD 2> /dev/null || true)
COMMIT := $(if $(shell git status --porcelain --untracked-files=no),"${COMMIT_NO}-dirty","${COMMIT_NO}")

RELEASE_DIR := $(CURDIR)/release

VERSION := ${shell cat ./VERSION}

SHELL := $(shell command -v bash 2>/dev/null)

.DEFAULT: runvm

runvm: $(SOURCES)
	go build -i $(EXTRA_FLAGS) -ldflags "-X main.gitCommit=${COMMIT} -X main.version=${VERSION} $(EXTRA_LDFLAGS)" -tags "$(BUILDTAGS)" -o runvm .

all: runvm


lint:
	go vet $(allpackages)
	go fmt $(allpackages)

install:
	install -D -m0755 runvm $(BINDIR)/runvm


uninstall:
	rm -f $(BINDIR)/runvm


clean:
	rm -f runvm
	rm -f contrib/cmd/recvtty/recvtty
	rm -rf $(RELEASE_DIR)

