FROM golang:alpine as base

WORKDIR /go/src/github.com/sethp/ci-experiments

COPY go.mod .
COPY go.sum .
RUN --mount=type=cache,target=/go/pkg/mod go mod download

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

COPY . .

# TODO: is it possible to group tidy, shellcheck, and golangci-lint in here but still run in parallel?
# would be even nicer if they could compose somewhat

FROM base as tidy

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/.cache/golangci-lint \
    ./hack/check-go-mod.sh

FROM base as shellcheck

COPY --from=koalaman/shellcheck:latest /bin/shellcheck /bin

# TODO does shellcheck cache?
RUN shellcheck hack/*.sh

FROM base as golangci-lint

COPY --from=golangci/golangci-lint:latest-alpine /usr/bin/golangci-lint /usr/bin

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/.cache/golangci-lint \
    golangci-lint run

# TODO: --out-format=github-actions

FROM base as test

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go test .

# TODO: test report, coverage ?

FROM base as builder

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -o /app .

FROM gcr.io/distroless/static:nonroot as app

COPY --from=builder /app /app

USER 65532:65532
ENTRYPOINT ["/app"]
