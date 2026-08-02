# Build context is this directory (applications/finances), not the repo
# root - unlike applications/lumenetes, this module has no local "replace"
# directive back to the root module, so it's fully self-contained.
#
# Generated protobuf/Connect code (gen/, web/src/gen/) is committed to git
# (see .gitignore's comment), so neither stage below needs buf/protoc - they
# just build against what's already there. Run `make gen` after editing
# proto/ to regenerate it.

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
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist ./internal/webui/dist

ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
RUN go build -a -ldflags '-s -w' -o /finances ./cmd/finances

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /finances /finances

USER 65534:65534

ENTRYPOINT ["/finances"]
