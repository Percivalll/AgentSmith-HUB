# AgentSmith-HUB Dockerfile
# Unified image for both leader and follower modes

FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    libc6-compat \
    nginx \
    bash \
    curl \
    wget

# Create non-root user
RUN addgroup -g 1000 agentsmith && \
    adduser -D -s /bin/bash -u 1000 -G agentsmith agentsmith

# Set working directory
WORKDIR /opt

# Copy and extract the deployment archive
ARG TARGETARCH
RUN rm -f agentsmith-hub* 2>/dev/null || true
COPY agentsmith-hub-*.tar.gz ./
RUN echo "=== STEP 1: Extract tar.gz ===" && \
    tar -xzf agentsmith-hub-${TARGETARCH}.tar.gz && \
    echo "=== STEP 2: Check extracted structure ===" && \
    ls -la && \
    echo "=== STEP 3: Check agentsmith-hub directory ===" && \
    ls -la agentsmith-hub/ && \
    echo "=== STEP 4: Remove tar.gz ===" && \
    rm agentsmith-hub-*.tar.gz && \
    echo "=== STEP 5: Final structure ===" && \
    ls -la && \
    chmod +x ./agentsmith-hub/agentsmith-hub

# Ensure startup scripts are executable
RUN chmod +x ./agentsmith-hub/start.sh ./agentsmith-hub/stop.sh

# Create docker-entrypoint.sh with frontend and backend
RUN echo '#!/bin/bash' > ./docker-entrypoint.sh && \
    echo 'set -e' >> ./docker-entrypoint.sh && \
    echo '' >> ./docker-entrypoint.sh && \
    echo '# Start nginx for frontend' >> ./docker-entrypoint.sh && \
    echo 'nginx -g "daemon off;" &' >> ./docker-entrypoint.sh && \
    echo '' >> ./docker-entrypoint.sh && \
    echo '# Start backend' >> ./docker-entrypoint.sh && \
    echo 'if [ "$MODE" = "follower" ]; then' >> ./docker-entrypoint.sh && \
    echo '  exec ./agentsmith-hub/start.sh --follower' >> ./docker-entrypoint.sh && \
    echo 'else' >> ./docker-entrypoint.sh && \
    echo '  exec ./agentsmith-hub/start.sh' >> ./docker-entrypoint.sh && \
    echo 'fi' >> ./docker-entrypoint.sh && \
    chmod +x ./docker-entrypoint.sh

# Create necessary directories
RUN mkdir -p /tmp/hub_logs /var/lib/nginx/html /var/log/nginx

# Configure nginx (nginx.conf is now available from the extracted archive)
RUN cp agentsmith-hub/nginx/nginx.conf /etc/nginx/nginx.conf

# Set proper ownership
RUN chown -R agentsmith:agentsmith /opt/agentsmith-hub /tmp/hub_logs /var/lib/nginx /var/log/nginx /etc/nginx/nginx.conf

# Set environment variables
ENV CONFIG_ROOT=/opt/agentsmith-hub/config
ENV LOG_LEVEL=info
ENV NODE_ID=default
ENV MODE=leader
ENV SIMD_ENABLED=false

# Expose ports
EXPOSE 8080 80

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ping || exit 1

# Switch to non-root user
USER agentsmith

# Copy LICENSE file into the image
COPY LICENSE /opt/LICENSE

# Default command
ENTRYPOINT ["./docker-entrypoint.sh"]
