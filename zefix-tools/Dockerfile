# syntax=docker/dockerfile:1.23

ARG BINARY=zefix-import

FROM golang:1.26-alpine AS builder
ARG BINARY
ENV CGO_ENABLED=0 \
    GOFLAGS="-buildvcs=false -mod=mod"

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -trimpath -ldflags="-s -w" -o /out/app "./cmd/${BINARY}"

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=builder /out/app /usr/local/bin/app
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/app"]
