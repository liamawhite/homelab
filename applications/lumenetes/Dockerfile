# Build context is the REPO ROOT, not this directory (see Makefile) -
# go.mod's "replace github.com/liamawhite/homelab => ../../" needs the root
# module's source tree present at the same relative offset, so the whole
# repo has to be in the build context. This also means the usual
# "COPY go.mod go.sum, go mod download, then COPY the rest" layer-caching
# trick doesn't apply here: `go mod download` can't resolve a local-path
# replace before that path's source actually exists in the image, so
# everything is copied up front instead.
#
# One image, both of this module's cmd/ binaries: lumenetes-controller and
# hub-controller share almost their entire codebase (api/v1alpha1,
# internal/bridges, the pkg/lumenetes/hue reuse above) - building/pushing two
# separately-tagged images for them would just double the build time and
# registry bookkeeping (including GHCR's per-package visibility setting)
# for no benefit, since what actually separates the two at runtime is the
# Deployment (ServiceAccount, RBAC, hostNetwork, command:), not the image.
#
# lumenetes-controller also serves the read-only web UI itself (see
# internal/server, internal/webui, cmd/lumenetes-controller/main.go) rather
# than that being a separate binary/image/Deployment - it already holds the
# Kubernetes client and RBAC the UI's Connect API needs to read
# Light/Switch/Group/Scene/CircadianSchedule/HueBridge, so a second
# container would only duplicate that for no benefit. The web-builder stage
# below produces the built frontend that gets embedded into it.
FROM node:22-alpine AS web-builder
WORKDIR /app/web
COPY applications/lumenetes/web/package.json applications/lumenetes/web/package-lock.json ./
RUN npm ci
COPY applications/lumenetes/web/ .
RUN npm run build

FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY . .
COPY --from=web-builder /app/web/dist /app/applications/lumenetes/internal/webui/dist

WORKDIR /app/applications/lumenetes
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=arm64
RUN go build -a -ldflags '-s -w' -o /lumenetes-controller ./cmd/lumenetes-controller
RUN go build -a -ldflags '-s -w' -o /hub-controller ./cmd/hub-controller

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /lumenetes-controller /lumenetes-controller
COPY --from=builder /hub-controller /hub-controller

USER 65534:65534

# No ENTRYPOINT - each Deployment sets `command:` to pick which binary to
# run (see pkg/components/lumenetescontroller and pkg/components/hubcontroller).
