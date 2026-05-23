# Stage 1: Download and cache dependencies
FROM golang:1.24-alpine AS deps
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Stage 2: Run tests (used by docker compose test)
FROM deps AS test
COPY . .
RUN go vet ./...
CMD ["go", "test", "./...", "-v", "-count=1"]

# Stage 3: Compile the production binary
FROM deps AS builder
WORKDIR /app
COPY . .
ARG APP_VERSION=dev
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -ldflags="-s -w -X main.Version=${APP_VERSION}" -o main .

# Stage 4: Minimal runtime image
FROM alpine:3.19 AS runner
WORKDIR /app
LABEL org.opencontainers.image.title="Go Distributed Rate Limiter"
LABEL org.opencontainers.image.source="https://github.com/sha-wrks/Go-Limiter"
LABEL org.opencontainers.image.licenses="MIT"
RUN addgroup --system --gid 1001 gopher && \
    adduser  --system --uid 1001 gopher
COPY --from=builder --chown=gopher:gopher /app/main ./
ENV APP_ENV=production \
    REDIS_ADDR=redis:6379 \
    PORT=8080
USER gopher
EXPOSE 8080
CMD ["./main"]
