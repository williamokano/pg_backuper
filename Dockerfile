FROM golang:1.22-bullseye AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o pg_backuper .

FROM debian:bullseye-slim

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
    wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add - && \
    echo "deb http://apt.postgresql.org/pub/repos/apt/ bullseye-pgdg main" > /etc/apt/sources.list.d/pgdg.list && \
    apt-get update && \
    apt-get install -y jq postgresql-client-${POSTGRES_VERSION} && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the pre-built binary and configuration files from the builder stage
COPY --from=builder /app/pg_backuper /usr/local/bin/pg_backuper
COPY db_config_schema.json ./db_config_schema.json
COPY noop_config.json ./noop_config.json
COPY entrypoint.sh ./entrypoint.sh
COPY docker_bin.sh ./docker_bin.sh

# Copy the crontab file and set up cron
COPY crontab /etc/cron.d/pg_backuper-cron
RUN chmod 0644 /etc/cron.d/pg_backuper-cron

# Set environment variables for cron schedule and PostgreSQL version
# Default schedule: every day at 3 AM
ENV CRON_SCHEDULE="0 3 * * *"
ENV CONFIG_FILE="/app/noop_config.json"
# Default PostgreSQL client version
ENV POSTGRES_VERSION=${POSTGRES_VERSION}

RUN chmod +x /app/entrypoint.sh
RUN chmod +x /app/docker_bin.sh
# Set the entrypoint to the Go app
ENTRYPOINT ["/app/entrypoint.sh"]