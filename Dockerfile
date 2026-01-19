FROM golang:1.24-bullseye AS builder
LABEL maintainer="Arie Brainware"

WORKDIR /src

# Install build deps
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    ca-certificates \
 && rm -rf /var/lib/apt/lists/*

# Cache dependencies
COPY go.mod go.sum ./
# If a vendor directory is present in the repo, prefer it and skip network fetches.
# Otherwise attempt to download modules (may fail if builder has no network).
RUN if [ -d ./vendor ]; then \
            echo "Using vendor directory, skipping go mod download"; \
        else \
            echo "No vendor dir found — attempting go mod download"; \
            go mod download; \
        fi

# Copy source and build the binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "-s -w" -o /basis-data-ltt ./

FROM alpine:latest
LABEL maintainer="Arie Brainware"

WORKDIR /app

# Timezone: rely on Go's embedded tzdata (import _ "time/tzdata").
# Avoid installing tzdata in Alpine to prevent network/package repo errors.
ENV TZ=Asia/Jakarta

# Runtime environment variables (can be set at docker build or run time)
ARG APPNAME
ARG APITOKEN
ARG APPENV
ARG APPPORT
ARG GINMODE
ARG DBHOST
ARG DBPORT
ARG DBNAME
ARG DBUSER
ARG DBPASS
ARG JWTSECRET
ARG REDIS_HOST
ARG REDIS_PORT

ENV APPNAME=$APPNAME \
    APITOKEN=$APITOKEN \
    APPENV=$APPENV \
    APPPORT=$APPPORT \
    GINMODE=$GINMODE \
    DBHOST=$DBHOST \
    DBPORT=$DBPORT \
    DBNAME=$DBNAME \
    DBUSER=$DBUSER \
    DBPASS=$DBPASS \
    JWTSECRET=$JWTSECRET \
    REDIS_HOST=$REDIS_HOST \
    REDIS_PORT=$REDIS_PORT

# Copy binary from builder
COPY --from=builder /basis-data-ltt ./basis-data-ltt
RUN chmod +x ./basis-data-ltt

EXPOSE 19091

CMD ["./basis-data-ltt"]