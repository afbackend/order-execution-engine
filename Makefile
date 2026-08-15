.PHONY: help run test race cover vet lint fmt fmt-check tidy check clean

.DEFAULT_GOAL := help

BIN := bin/riskd
STATICCHECK := honnef.co/go/tools/cmd/staticcheck@2025.1.1

help:       ## list available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  %-10s %s\n", $$1, $$2}'

# Silenced with @ because stdout is a JSON Lines stream: an echoed recipe would
# corrupt it. Built rather than `go run` so the exit code is the daemon's own.
run:        ## build and run the daemon (Ctrl-C to stop; JSONL on stdout, logs on stderr)
	@go build -o $(BIN) .
	@$(BIN)

test:       ## run all tests
	go test ./...

race:       ## run tests with the race detector (concurrency safety)
	go test -race ./...

cover:      ## run tests and print a per-function coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

vet:        ## static analysis built into the toolchain
	go vet ./...

lint:       ## staticcheck (pinned version, no install step needed)
	go run $(STATICCHECK) ./...

fmt:        ## format all code (gofmt is non-negotiable in Go)
	go fmt ./...

fmt-check:  ## fail if any file is not gofmt-formatted (does not rewrite)
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then \
		echo "These files are not gofmt-formatted:"; \
		echo "$$out"; \
		exit 1; \
	fi

tidy:       ## sync go.mod / go.sum
	go mod tidy

clean:      ## remove build and coverage artifacts
	rm -rf bin coverage.out

check: fmt-check vet lint race ## the pre-commit gate: CI runs exactly this target
