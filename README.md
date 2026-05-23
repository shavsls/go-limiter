# Go Distributed Rate Limiter

[![CI](https://github.com/sha-wrks/Go-Limiter/actions/workflows/ci.yml/badge.svg)](https://github.com/sha-wrks/Go-Limiter/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.24-00ADD8?logo=go)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/docker-compose-2496ED?logo=docker)](docker-compose.yml)

A production-ready distributed rate-limiting middleware written in Go 1.24. Uses atomic Lua scripts on Redis to enforce per-client request limits across multiple application instances without race conditions.

---

## Features

- **Atomic counters** - single Lua script per request; no lost updates across concurrent instances
- **Configurable** - limit and window driven by environment variables, no recompile needed
- **Standard headers** - `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After` on every response
- **Proxy-aware** - resolves real client IP from `X-Real-IP` / `X-Forwarded-For` before falling back to `RemoteAddr`
- **Minimal runtime** - static binary in a 7 MB Alpine image running as a non-root user
- **Integration tested** - tests run against a live Redis instance inside Docker Compose

---

## Architecture

```
  Client
    │
    ▼
┌──────────┐   HTTP    ┌──────────────────────────────────────┐
│  Reverse │ ────────► │           Go HTTP Server              │
│  Proxy   │           │                                       │
└──────────┘           │  /data  ──► RateLimiter.Middleware()  │
                       │               │                       │
                       │               ▼                       │
                       │         Redis (Lua INCR + EXPIRE)     │
                       │               │                       │
                       │        allowed? ──► handler()         │
                       │        denied?  ──► 429               │
                       └──────────────────────────────────────-┘
```

**Rate-limiting algorithm:**

1. `INCR rate_limit:<ip>` - atomically increment the counter
2. If this is the first increment, `EXPIRE` the key to the configured window duration
3. Compare counter to limit → allow or reject
4. Return current count to the middleware for remaining-count headers

---

## Quick Start

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (includes Compose)

### Run

```bash
# 1. Clone
git clone https://github.com/sha-wrks/Go-Limiter.git
cd Go-Limiter

# 2. Copy config
cp .env.example .env

# 3. Start the stack (app + Redis)
docker compose up --build -d

# 4. Verify - send 7 requests (limit is 5 per 30 s by default)
for i in {1..7}; do curl -si http://localhost:8080/data | grep -E "^HTTP|X-RateLimit|Retry-After"; echo "---"; done
```

Expected output:

```
HTTP/1.1 200 OK
X-RateLimit-Limit: 5
X-RateLimit-Remaining: 4
---
...
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 5
X-RateLimit-Remaining: 0
Retry-After: 30
---
```

---

## Configuration

All settings are read from environment variables (see [`.env.example`](.env.example)):

| Variable            | Default          | Description                                     |
|---------------------|------------------|-------------------------------------------------|
| `REDIS_ADDR`        | `localhost:6379` | Redis address. Use `redis:6379` inside Docker   |
| `PORT`              | `8080`           | HTTP listen port                                |
| `RATE_LIMIT`        | `5`              | Max requests allowed per window                 |
| `RATE_LIMIT_WINDOW` | `30s`            | Window duration (Go duration string: `30s`, `1m`, `5m`) |
| `APP_VERSION`       | `dev`            | Injected at build time via `-X main.Version`    |

---

## API Reference

### `GET /data`

Returns a plain-text response. Subject to rate limiting.

**Response headers (all replies):**

| Header                  | Description                          |
|-------------------------|--------------------------------------|
| `X-RateLimit-Limit`     | Max requests allowed per window      |
| `X-RateLimit-Remaining` | Requests remaining in current window |

**Additional header on `429`:**

| Header        | Description                                   |
|---------------|-----------------------------------------------|
| `Retry-After` | Seconds to wait before retrying (window size) |

**Status codes:**

| Code  | Meaning                         |
|-------|---------------------------------|
| `200` | Request processed               |
| `429` | Rate limit exceeded             |

---

### `GET /healthz`

Returns `200 ok`. No dependencies - safe for liveness probes.

---

## Testing

Tests are integration tests that connect to a real Redis instance. They **skip** automatically when Redis is unavailable, so they are safe to run in any environment.

### Run via Docker Compose (recommended)

```bash
# Builds the test image, runs tests against a fresh Redis, then tears everything down
make test-docker
```

Under the hood this runs:
```bash
docker compose --profile test up --build --abort-on-container-exit --exit-code-from test test redis
docker compose --profile test down -v
```

### Manual smoke test

```bash
make up       # start the stack
make validate # fire 7 requests and inspect headers
make clean    # stop and remove containers, volumes, and images
```

---

## Container Architecture

The Dockerfile uses a **4-stage build** to minimize image size and attack surface:

| Stage     | Base image           | Purpose                                      |
|-----------|----------------------|----------------------------------------------|
| `deps`    | `golang:1.24-alpine` | Download modules - layer cached for rebuilds |
| `test`    | ← `deps`             | `go vet` + `go test ./...`                   |
| `builder` | ← `deps`             | Compile static binary (`-s -w`)              |
| `runner`  | `alpine:3.19`        | Runtime - non-root `gopher` user, ~7 MB      |

---

## CI / CD

GitHub Actions runs on every push and pull request to `main`:

1. **Test** - spins up a Redis service container, runs `go test ./... -race -cover`
2. **Docker Build** - builds the production image with Buildx layer caching

---

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

1. Fork → branch → commit → push → PR
2. Follow the existing code style (`go vet` must pass)
3. Include tests for new behavior

Report bugs or suggest features via [GitHub Issues](https://github.com/sha-wrks/Go-Limiter/issues).

---

## License

[MIT](LICENSE) © 2026 Yansha
