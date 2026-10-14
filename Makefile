.PHONY: all help web build test check notices dist release clean

VERSION := $(shell tr -d ' \r\n' < VERSION)
EXE := $(shell go env GOEXE)

all: build

help:
	@echo "Designer make targets:"
	@echo "  make build              build the frontend and bin/designer for this machine"
	@echo "  make web                build the frontend only (web/dist, embedded in the binary)"
	@echo "  make test               go vet + go test (needs web/dist)"
	@echo "  make check              everything CI runs: frontend, vet, tests, notices"
	@echo "  make notices            regenerate THIRD_PARTY_NOTICES.md"
	@echo "  make dist               release archives for every platform into dist/"
	@echo "  make release V=0.2.0    set VERSION, commit and tag v0.2.0 (then push the tag)"
	@echo "  make clean              remove build output"

web:
	cd web && npm ci && npm run check-icons && npm run build

build: web
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/designer$(EXE) ./cmd/designer
	@echo "built bin/designer$(EXE) $(VERSION)"

test:
	go vet ./...
	go test ./...
	cd web && npm test

notices:
	node scripts/third-party-notices.mjs

check: web test
	node scripts/third-party-notices.mjs --check

dist:
	VERSION=$(VERSION) scripts/dist.sh

release:
	@test -n "$(V)" || { echo "usage: make release V=0.2.0"; exit 1; }
	@test -z "$$(git status --porcelain)" || { echo "commit or stash your changes first"; exit 1; }
	@grep -q "^## \[$(V)\]" CHANGELOG.md || { echo "add a '## [$(V)]' section to CHANGELOG.md first"; exit 1; }
	echo "$(V)" > VERSION
	git add VERSION
	git commit -m "Release v$(V)"
	git tag -a "v$(V)" -m "Release v$(V)"
	@echo "Tagged v$(V). Push it to publish: git push origin HEAD v$(V)"

clean:
	rm -rf bin dist web/dist
