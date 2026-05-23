.PHONY: build test test-docker up down logs clean lint vet validate

APP_VERSION ?= dev
BINARY      := bin/go-limiter

## ── Local build ────────────────────────────────────────────────────────────
build:
	go build -ldflags="-s -w -X main.Version=$(APP_VERSION)" -o $(BINARY) .

vet:
	go vet ./...

## ── Docker Compose: app stack ──────────────────────────────────────────────
up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f app

## ── Docker Compose: test suite (runs then exits) ───────────────────────────
test-docker:
	docker compose --profile test up --build --abort-on-container-exit --exit-code-from test test redis
	docker compose --profile test down -v

## ── Validation (manual smoke test after `make up`) ─────────────────────────
validate:
	@echo "Sending 7 requests to http://localhost:8080/data ..."
	@for i in 1 2 3 4 5 6 7; do \
		curl -si http://localhost:8080/data | grep -E "^HTTP|X-RateLimit|Retry-After"; \
		echo "---"; \
	done

## ── Cleanup ─────────────────────────────────────────────────────────────────
clean:
	docker compose down -v --rmi local
	rm -rf bin/ coverage.out coverage.html
