# --- Builder stage for static golang binary ---
FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN apt-get update && apt-get install -y sqlite3 gcc

# Build a fully static binary using musl.
RUN CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -tags netgo -ldflags '-w -extldflags "-static"' -o chore-tracker ./app

# --- Final Stage ---
FROM debian:bullseye-slim

ENV PATH="/opt/certbot/bin:${PATH}"

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates openssl curl certbot python3 python3-venv sqlite3 && \
    rm -rf /var/lib/apt/lists/*
    
RUN python3 -m venv /opt/certbot/
RUN /opt/certbot/bin/pip install --upgrade pip
RUN /opt/certbot/bin/pip install certbot certbot-dns-duckdns

# Create directories
WORKDIR /app
RUN mkdir -p /app/certbot/config /app/certbot/work /app/certbot/logs
RUN touch /app/duckdns.log

# create non-root user/group
RUN groupadd appgroup && useradd -m -u 1000 -g appgroup -s /bin/bash appuser

# Copy files
COPY --from=builder /app/chore-tracker /app/chore-tracker
COPY --from=builder /app/app/static /app/static
COPY --from=builder /app/app/templates /app/templates
COPY --from=builder /app/scripts /app/scripts
COPY --from=builder /app/db /app/db
COPY docker-entrypoint.sh /app/

# Set ownership and permissions,
RUN chown -R appuser:appgroup /app/certbot
RUN chown appuser:appgroup /app/duckdns.log
RUN chmod +rx /app/docker-entrypoint.sh

# switch to non-root user
USER appuser

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["./chore-tracker"]

EXPOSE 443