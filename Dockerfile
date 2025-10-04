# Build stage
FROM golang:1.21-alpine AS builder

# Set build arguments for flexible mirror configuration
ARG GOPROXY_PRIMARY="https://goproxy.cn,direct"
ARG GOPROXY_SECONDARY="https://mirrors.aliyun.com/goproxy/,direct"
ARG GOPROXY_TERTIARY="https://goproxy.io,direct"
ARG GOPROXY_DEFAULT="https://proxy.golang.org,direct"
ARG APK_MIRROR=""

WORKDIR /build

# Detect network location and configure mirrors intelligently
# Priority: CN mainland mirrors -> Official sources
RUN set -eux; \
    # Test network connectivity to determine optimal mirror
    if wget -q --spider --timeout=3 https://www.baidu.com 2>/dev/null; then \
        echo "Detected: China mainland network, using CN CDN mirrors"; \
        # Configure Alpine APK mirror for CN
        sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories; \
        # Set Go proxy with CN mirrors (fallback chain)
        export GOPROXY="${GOPROXY_PRIMARY}"; \
        echo "GOPROXY set to: ${GOPROXY}"; \
    else \
        echo "Detected: International network, using official sources"; \
        export GOPROXY="${GOPROXY_DEFAULT}"; \
        echo "GOPROXY set to: ${GOPROXY}"; \
    fi; \
    # Verify and display final configuration
    echo "Active APK repositories:"; \
    cat /etc/apk/repositories; \
    echo "Active GOPROXY: ${GOPROXY}"

# Copy dependency files
COPY go.mod go.sum ./

# Download dependencies with intelligent proxy selection
RUN set -eux; \
    # Primary attempt with CN mirror
    if wget -q --spider --timeout=3 https://www.baidu.com 2>/dev/null; then \
        export GOPROXY="${GOPROXY_PRIMARY}"; \
        echo "Using primary CN proxy: ${GOPROXY}"; \
        go mod download || { \
            echo "Primary proxy failed, trying secondary..."; \
            export GOPROXY="${GOPROXY_SECONDARY}"; \
            go mod download || { \
                echo "Secondary proxy failed, trying tertiary..."; \
                export GOPROXY="${GOPROXY_TERTIARY}"; \
                go mod download; \
            }; \
        }; \
    else \
        export GOPROXY="${GOPROXY_DEFAULT}"; \
        echo "Using official proxy: ${GOPROXY}"; \
        go mod download; \
    fi

# Copy source code
COPY . .

# Build binary with optimization flags
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o cursor2api main.go

# Runtime stage
FROM alpine:3.19

# Set APK mirror for runtime stage
ARG APK_MIRROR=""
RUN set -eux; \
    if wget -q --spider --timeout=3 https://www.baidu.com 2>/dev/null; then \
        echo "Detected: China mainland network, configuring CN APK mirror"; \
        sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories; \
    fi; \
    # Install CA certificates and wget for health checks
    apk --no-cache add ca-certificates wget

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder stage with proper ownership
COPY --from=builder --chown=appuser:appuser /build/cursor2api .

# Switch to non-root user
USER appuser

# Expose application port
EXPOSE 5680

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD wget --spider -q http://localhost:5680/health || exit 1

# Run application
ENTRYPOINT ["./cursor2api"]