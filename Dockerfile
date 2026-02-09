FROM golang:1.22-bookworm AS builder

WORKDIR /app
COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -o /out/batch ./cmd/batch

FROM golang:1.22-bookworm

WORKDIR /app

# Keep source tree in image because built-in transforms compile plugins
# from mrapps/*.go at runtime.
COPY --from=builder /out/batch /usr/local/bin/batch
COPY . /app

RUN mkdir -p /app/.cache/go-build /app/.cache/go-mod /app/.cache/go-tmp /app/output

ENV GO=/usr/local/go/bin/go \
    GOCACHE=/app/.cache/go-build \
    GOMODCACHE=/app/.cache/go-mod \
    GOTMPDIR=/app/.cache/go-tmp

ENTRYPOINT ["batch"]
