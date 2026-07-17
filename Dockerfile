# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY migrations/ ./migrations/
RUN rm -rf ./internal/webui/dist
COPY --from=frontend /src/web/out/ ./internal/webui/dist/

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILT_AT=1970-01-01T00:00:00Z
ARG DEPLOYMENT=container
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -buildvcs=false \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.builtAt=${BUILT_AT} -X main.deployment=${DEPLOYMENT}" \
      -o /out/navax ./cmd/navax

FROM alpine:3.22 AS runtime
# ffmpeg: video background compress + poster frame (background media library)
RUN apk add --no-cache ca-certificates tzdata ffmpeg \
    && addgroup -S -g 10001 navax \
    && adduser -S -D -H -u 10001 -G navax navax \
    && mkdir -p /data \
    && chown navax:navax /data \
    && chmod 0700 /data
COPY --from=backend --chown=navax:navax /out/navax /usr/local/bin/navax

ENV NAVAX_ADDR=:8080 \
    NAVAX_DATA_DIR=/data
USER 10001:10001
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/readyz || exit 1
ENTRYPOINT ["/usr/local/bin/navax"]
