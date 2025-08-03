# Scratch container for minimal attack surface
FROM scratch

# Copy ca-certificates from another image
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the pre-built binary
COPY bin/coredns /coredns

# Use non-root user by default
USER 65534:65534

# Expose DNS ports
EXPOSE 53 53/udp

# Set the binary as entrypoint
ENTRYPOINT ["/coredns"]
