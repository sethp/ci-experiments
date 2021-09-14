FROM golang:alpine as base

WORKDIR /go/src/github.com/sethp/ci-experiments

# TODO: better to do this or cache-mount /go/pkg/mod ?
# This means go.mod and go.sum changes cache-bust shellcheck, bash install, etc., let's leave the downloads implicit?
# COPY go.mod .
# COPY go.sum .
# RUN --mount=type=cache,target=/go/pkg/mod \
#     go mod download

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# TODO: this vs --mount=target=. ?
# COPY . .

# TODO: is it possible to group tidy, shellcheck, and golangci-lint in here but still run in parallel?
# would be even nicer if they could compose somewhat

FROM base as tidy

# TODO: possible to scope the `--mount=target` down?
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/.cache/golangci-lint \
    --mount=target=.,rw \
    ./hack/check-go-mod.sh

FROM base as shellcheck

# TODO does shellcheck cache?
# TODO: possible to scope the `--mount=target` down?
RUN --mount=from=koalaman/shellcheck:latest,source=/bin/shellcheck,target=/bin/shellcheck \
    --mount=target=. \
    shellcheck hack/*.sh

FROM base as golangci-lint

RUN --mount=from=golangci/golangci-lint:latest-alpine,target=/usr/bin/golangci-lint,source=/usr/bin/golangci-lint \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/.cache/golangci-lint \
    --mount=target=. \
    golangci-lint run

# TODO: --out-format=github-actions

FROM base as test

RUN mkdir /out
RUN --mount=type=cache,target=/var/cache/apk \
    apk add --update bash

SHELL ["/bin/bash", "-euo", "pipefail", "-c"]

ARG CI

# Ooh, this is a sticky one: it works great when tests pass, and fails appropriately when
# they don't, but *doesn't* emit test2json output in that case: we bomb out here, and the
# later "test-out" stage never gets a chance to run.
#
# see also: https://github.com/moby/buildkit/issues/1421
#      and: https://github.com/moby/buildkit/issues/1472
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=target=. \
    go test ./... "${CI+-v}" --coverprofile=/out/coverage.out | tee /dev/stderr | go tool test2json > /out/test.json

FROM scratch as test-out

# test report, coverage
COPY --from=test /out/ /

# Well, this is exactly what I was hoping to avoid, but here we are
FROM base as test-ci

RUN mkdir /out

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=target=. \
    ( go test ./... -v --coverprofile=/out/coverage.out ; echo $? > /tmp/exitcode ) | tee /dev/stderr | go tool test2json > /out/test.json

FROM scratch as test-ci-out

COPY --from=test-ci /out/ /

FROM busybox as test-ci-check

RUN --mount=from=test-ci,target=/tmp,source=/tmp \
    ec="$(cat /tmp/exitcode)"; \
    [ "$ec" = 0 ] || echo >&2 "tests failed! see out/test.json or re-run with \`--no-cache --progress=plain\` for more details" ; \
    exit "$ec"

FROM base as builder

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=target=. \
    go build -o /app .

# TODO: this would output even when tests fail; what's a good way to mark it as dependent?
# Include something that copies/mounts from all previous stages? To /dev/null?
FROM gcr.io/distroless/static:nonroot as app

COPY --from=builder /app /app

USER 65532:65532
ENTRYPOINT ["/app"]
