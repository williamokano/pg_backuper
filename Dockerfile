FROM golang:1.24-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o pg_backuper .

FROM debian:trixie-slim

ARG POSTGRES_VERSION=16

# Install dependencies and cron
RUN apt-get update && apt-get install -y \
    jq \
    cron \
    wget \
    gnupg \
    lsb-release \
    && rm -rf /var/lib/apt/lists/*

# Install required packages and PostgreSQL APT repository
RUN apt-get update && \
    apt-get install -y gnupg wget && \
    wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /usr/share/keyrings/postgresql-keyring.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/postgresql-keyring.gpg] http://apt.postgresql.org/pub/repos/apt/ trixie-pgdg main" > /etc/apt/sources.list.d/pgdg.list && \
    apt-get update && \
    apt-get install -y jq postgresql-client-${POSTGRES_VERSION} && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the pre-built binary and configuration files from the builder stage
COPY --from=builder /app/pg_backuper /usr/local/bin/pg_backuper
COPY db_config_schema.json ./db_config_schema.json
COPY noop_config.json ./noop_config.json
COPY entrypoint.sh ./entrypoint.sh

# Copy the crontab file and set up cron
COPY crontab /etc/cron.d/pg_backuper-cron
RUN chmod 0644 /etc/cron.d/pg_backuper-cron

# Set environment variables
# Cron runs hourly; smart scheduling in tool decides if backup is due
ENV CONFIG_FILE="/app/noop_config.json"
# Default PostgreSQL client version
ENV POSTGRES_VERSION=${POSTGRES_VERSION}

RUN chmod +x /app/entrypoint.sh
# Set the entrypoint to the Go app
ENTRYPOINT ["/app/entrypoint.sh"]