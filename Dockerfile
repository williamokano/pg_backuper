FROM golang:1.22-bullseye AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o pg_backuper .

FROM debian:bullseye-slim

# Install required packages and PostgreSQL APT repository
RUN apt-get update && \
    apt-get install -y gnupg wget && \
    wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add - && \
    echo "deb http://apt.postgresql.org/pub/repos/apt/ bullseye-pgdg main" > /etc/apt/sources.list.d/pgdg.list && \
    apt-get update && \
    apt-get install -y jq postgresql-client-16 && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the pre-built binary and configuration files from the builder stage
COPY --from=builder /app/pg_backuper /usr/local/bin/pg_backuper
COPY db_config_schema.json ./db_config_schema.json
COPY noop_config.json ./noop_config.json

# Set the entrypoint to the Go app
ENTRYPOINT ["/usr/local/bin/pg_backuper"]