# Build context is the REPO ROOT, not this directory (see Makefile) -
# go.mod's "replace github.com/liamawhite/homelab => ../../" needs the root
# module's source tree present at the same relative offset, so the whole
# repo has to be in the build context. This also means the usual
# "COPY go.mod go.sum, go mod download, then COPY the rest" layer-caching
# trick doesn't apply here: `go mod download` can't resolve a local-path
# replace before that path's source actually exists in the image, so
# everything is copied up front instead.
#
# One image, both of this module's cmd/ binaries: lights-controller and
# hub-controller share almost their entire codebase (api/v1alpha1,
# internal/bridges, the pkg/lights/hue reuse above) - building/pushing two
# separately-tagged images for them would just double the build time and
# registry bookkeeping (including GHCR's per-package visibility setting)
# for no benefit, since what actually separates the two at runtime is the
# Deployment (ServiceAccount, RBAC, hostNetwork, command:), not the image.
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY . .

WORKDIR /app/applications/lights-controller
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=arm64
RUN go build -a -ldflags '-s -w' -o /lights-controller ./cmd/lights-controller
RUN go build -a -ldflags '-s -w' -o /hub-controller ./cmd/hub-controller

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /lights-controller /lights-controller
COPY --from=builder /hub-controller /hub-controller

USER 65534:65534

# No ENTRYPOINT - each Deployment sets `command:` to pick which binary to
# run (see pkg/components/lightscontroller and pkg/components/hubcontroller).
