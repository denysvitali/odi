# syntax=docker/dockerfile:1.7

# Build stage — uses gocv's official image which ships OpenCV 4.10 + build toolchain.
FROM gocv/opencv:4.10.0 AS builder

ENV CGO_ENABLED=1 \
    GOFLAGS="-buildvcs=false"

# Install a recent Go (the base image's Go may lag behind go.mod's toolchain).
ARG GO_VERSION=1.26.0
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tgz \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf /tmp/go.tgz \
    && rm /tmp/go.tgz
ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -trimpath -ldflags="-s -w" -o /out/odi-backend .

# Runtime stage — keep the same OpenCV base so shared libs match exactly.
FROM gocv/opencv:4.10.0 AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd --system --gid 10001 odi \
    && useradd  --system --uid 10001 --gid odi --home-dir /app --shell /usr/sbin/nologin odi

WORKDIR /app
COPY --from=builder /out/odi-backend /usr/local/bin/odi-backend

USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/odi-backend"]
CMD ["serve"]
