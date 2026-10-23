# syntax=docker/dockerfile:1
#
# cragbook from a clone: `docker build -t cragbook .`, or `docker compose --profile app up`.
# The frontend built by Node, a static Go binary, the CA bundle for cragcast's HTTPS — no shell,
# non-root. Node and Go run on the build machine and Go cross-compiles (no cgo), so an arm64
# image builds on amd64 without emulating anything; the last stage only copies files.

FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /web
RUN npm install -g pnpm@12.14.0
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM --platform=$BUILDPLATFORM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/cragbook ./cmd/cragbook

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/cragbook /cragbook
COPY --from=web /web/dist /web
# 65532 is distroless's nonroot; scratch has no /etc/passwd, so it is numeric.
USER 65532:65532
ENV CRAGBOOK_ADDR=0.0.0.0:8080 CRAGBOOK_WEB_DIR=/web
EXPOSE 8080
ENTRYPOINT ["/cragbook"]
CMD ["serve"]
