ARG NODE_VERSION=24.19.0
ARG GO_VERSION=1.27.1
ARG ALPINE_VERSION=3.22

FROM node:${NODE_VERSION}-alpine AS web-builder

WORKDIR /src/web

RUN npm install --global pnpm@11.22.0

COPY web/package.json web/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm build

FROM golang:${GO_VERSION}-alpine AS go-builder

ARG BINARY_NAME=one-more-round
ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
COPY --from=web-builder /src/web/dist ./web/dist
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
      -ldflags "-s -w -X github.com/miebyte/goutils/buildinfo.Version=${VERSION} -X github.com/miebyte/goutils/buildinfo.ServiceName=${BINARY_NAME}" \
      -o /out/${BINARY_NAME} .

FROM alpine:${ALPINE_VERSION}

ARG BINARY_NAME=one-more-round

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app \
    && mkdir -p /app/data/photos \
    && chown -R app:app /app

WORKDIR /app

COPY --from=go-builder /out/${BINARY_NAME} /app/one-more-round

USER app

EXPOSE 8080
VOLUME ["/app/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/health_check || exit 1

ENTRYPOINT ["/app/one-more-round"]
CMD ["--configFile", "/app/config.json", "--listen", "0.0.0.0:8080"]
