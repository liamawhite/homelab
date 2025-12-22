# Build stage for static binary
FROM golang:1.24-alpine AS builder

# Install ca-certificates for build
RUN apk add --no-cache ca-certificates git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build static binary with all optimizations
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=arm64
RUN go build -a -ldflags '-extldflags "-static" -s -w' -o coredns .

# Runtime stage using scratch for minimal image
FROM scratch

# Copy ca-certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the static binary from build stage
COPY --from=builder /app/coredns /coredns

# Use non-root user by default
USER 65534:65534

# Expose DNS ports
EXPOSE 53 53/udp

# Set the binary as entrypoint
ENTRYPOINT ["/coredns"]
