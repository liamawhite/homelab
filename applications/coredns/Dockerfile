# Build stage with C dependencies
FROM golang:1.24-bookworm AS builder

# Install libunbound development libraries and build tools
RUN apt-get update && apt-get install -y \
    libunbound-dev \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Copy source code
COPY . .

# Enable CGO and build the binary
ENV CGO_ENABLED=1
RUN go build -o coredns .

# Runtime stage with minimal dependencies
FROM debian:bookworm-slim

# Install runtime libraries
RUN apt-get update && apt-get install -y \
    ca-certificates \
    libunbound8 \
    && rm -rf /var/lib/apt/lists/*

# Copy the binary from build stage
COPY --from=builder /app/coredns /coredns

# Use non-root user by default
USER 65534:65534

# Expose DNS ports
EXPOSE 53 53/udp

# Set the binary as entrypoint
ENTRYPOINT ["/coredns"]
