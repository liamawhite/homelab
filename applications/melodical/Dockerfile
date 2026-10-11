FROM --platform=$BUILDPLATFORM node:22.23.3-alpine AS frontend
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26.8-alpine AS backend
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/web/dist ./web/dist
RUN CGO_ENABLED=0 go test ./cmd/... ./internal/... ./web
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /melodical ./cmd/melodical

FROM scratch
COPY --from=backend /melodical /melodical
COPY --from=backend --chown=65532:65532 /build/runtime-data /data
USER 65532:65532
ENV MELODICAL_ADDR=:8080 MELODICAL_DB=/data/melodical.db
EXPOSE 8080
ENTRYPOINT ["/melodical"]
