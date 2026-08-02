# Build context is this directory (applications/home), not the repo root -
# like applications/reminders, this module has no local "replace" directive
# back to the root module, so it's fully self-contained.

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
COPY go.mod ./
COPY . .
COPY --from=web-builder /app/web/dist ./internal/webui/dist

ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
RUN go build -a -ldflags '-s -w' -o /home-server ./cmd/home

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /home-server /home-server

USER 65534:65534

ENTRYPOINT ["/home-server"]
