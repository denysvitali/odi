# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS builder
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
    go build -trimpath -ldflags="-s -w" -o /out/odi .

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=builder /out/odi /usr/local/bin/odi
USER nonroot:nonroot
EXPOSE 8085
ENTRYPOINT ["/usr/local/bin/odi"]
CMD ["serve"]
