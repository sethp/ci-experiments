FROM golang:alpine as base

WORKDIR /go/src/github.com/sethp/ci-experiments

# TODO: better to do this or cache-mount /go/pkg/mod ?
COPY go.mod .
COPY go.sum .
RUN --mount=type=cache,target=/go/pkg/mod go mod download

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

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=target=. \
    go test .

# TODO: test report, coverage ?

FROM base as builder

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=target=. \
    go build -o /app .

FROM gcr.io/distroless/static:nonroot as app

COPY --from=builder /app /app

USER 65532:65532
ENTRYPOINT ["/app"]
