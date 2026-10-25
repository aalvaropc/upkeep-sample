GO ?= go
BIN ?= bin/notesd

.PHONY: check fmt-check tidy-check vet test build smoke

check: fmt-check tidy-check vet test build

fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

tidy-check:
	$(GO) mod tidy -diff

vet:
	$(GO) vet ./...

test:
	$(GO) test -race -count=1 -shuffle=on ./...

build:
	CGO_ENABLED=0 $(GO) build -trimpath -o $(BIN) ./cmd/notesd

smoke: build
	@set -e; export NOTES_ADDR=127.0.0.1:18081 NOTES_JWT_SECRET="$$(openssl rand -hex 32)"; \
	./$(BIN) & pid=$$!; trap 'kill -TERM $$pid 2>/dev/null || true' EXIT; \
	base=http://$$NOTES_ADDR; \
	curl -fsS --retry 20 --retry-delay 1 --retry-all-errors $$base/healthz; \
	token="$$(./$(BIN) token -sub smoke -scope notes:write)"; \
	curl -fsS -X POST -H "Authorization: Bearer $$token" -H 'Content-Type: application/json' -d '{"title":"smoke"}' $$base/v1/notes; echo; \
	curl -fsS -H "Authorization: Bearer $$token" $$base/v1/notes; echo; \
	kill -TERM $$pid; wait $$pid; trap - EXIT
