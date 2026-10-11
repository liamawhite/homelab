FROM --platform=$BUILDPLATFORM node:22-alpine AS web-builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.25.8-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist /app/internal/webui/dist

ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
RUN go build -trimpath -ldflags '-s -w' -o /lumenetes-controller ./cmd/lumenetes-controller
RUN go build -trimpath -ldflags '-s -w' -o /hub-controller ./cmd/hub-controller

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /lumenetes-controller /lumenetes-controller
COPY --from=builder /hub-controller /hub-controller

USER 65534:65534

# No ENTRYPOINT - each Deployment sets `command:` to pick which binary to
# run (see pkg/components/lumenetescontroller and pkg/components/hubcontroller).
