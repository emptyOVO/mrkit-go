FROM golang:1.24-bullseye

WORKDIR /app

# Keep docs commands working in container examples.
ENV GO=/usr/local/go/bin/go
ENV GOCACHE=/tmp/.cache/go-build
ENV GOMODCACHE=/tmp/.cache/go-mod
ENV GOTMPDIR=/tmp/.cache/go-tmp

RUN apt-get update \
    && apt-get install -y --no-install-recommends bash ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /tmp/.cache/go-build /tmp/.cache/go-mod /tmp/.cache/go-tmp

COPY . .
