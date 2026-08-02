# Build context is THIS directory (applications/lumenetes), not the repo
# root - this module used to need `replace github.com/liamawhite/homelab =>
# ../../` (dropped in favor of a root-level go.work) and pkg/lumenetes/hue
# import from the root module (moved to internal/hue, in this module now),
# both of which forced the whole repo into the build context. Neither
# cmd/lumenetes-controller nor cmd/hub-controller import anything from the
# root module anymore (only cmd/lumenetes-lights, this module's CLI, still
# does - for pkg/config's infra.yaml loader - but that's not part of this
# image), so this module is fully self-contained for build purposes. The
# usual "COPY go.mod go.sum, go mod download, then COPY the rest"
# layer-caching trick still doesn't apply cleanly here since this module's
# go.mod itself still references the root module (for lumenetes-lights) -
# `go mod download` would try to resolve that even though these two binaries
# don't need it - so everything is still copied up front rather than
# splitting the go.mod/go.sum layer out.
#
# One image, both of this module's cmd/ binaries: lumenetes-controller and
# hub-controller share almost their entire codebase (api/v1alpha1,
# internal/bridges, internal/hue) - building/pushing two separately-tagged
# images for them would just double the build time and registry bookkeeping
# (including GHCR's per-package visibility setting) for no benefit, since
# what actually separates the two at runtime is the Deployment
# (ServiceAccount, RBAC, hostNetwork, command:), not the image.
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
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

FROM golang:1.25-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY . .
COPY --from=web-builder /app/web/dist /app/internal/webui/dist

ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
RUN go build -a -ldflags '-s -w' -o /lumenetes-controller ./cmd/lumenetes-controller
RUN go build -a -ldflags '-s -w' -o /hub-controller ./cmd/hub-controller

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /lumenetes-controller /lumenetes-controller
COPY --from=builder /hub-controller /hub-controller

USER 65534:65534

# No ENTRYPOINT - each Deployment sets `command:` to pick which binary to
# run (see pkg/components/lumenetescontroller and pkg/components/hubcontroller).
