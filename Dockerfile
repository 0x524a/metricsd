# Use Debian-based builder for better CGO/library compatibility
FROM golang:1.25-bookworm AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    make \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Build with all features including GPU support (NVML)
RUN go build -ldflags '-w -s' -o metricsd cmd/metricsd/main.go

FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -g 1000 metricsd && \
    useradd -r -u 1000 -g metricsd -s /bin/false metricsd

# Create directories
RUN mkdir -p /etc/metricsd/certs /var/lib/metricsd
RUN chown -R metricsd:metricsd /etc/metricsd /var/lib/metricsd

WORKDIR /home/metricsd

# Copy binary
COPY --from=builder /app/metricsd /usr/local/bin/metricsd
RUN chmod +x /usr/local/bin/metricsd

# Switch to non-root user
USER metricsd

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/metricsd"]
CMD ["-config", "/etc/metricsd/config.json"]
