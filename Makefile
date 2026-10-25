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
	@NOTES_ADDR=127.0.0.1:18081 ./$(BIN) & pid=$$!; \
	curl -fsS --retry 20 --retry-delay 1 --retry-all-errors http://127.0.0.1:18081/healthz; status=$$?; \
	kill -TERM $$pid; wait $$pid; exit $$status
