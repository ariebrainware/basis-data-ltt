FROM golang:1.24-alpine AS builder
LABEL maintainer="Arie Brainware"

WORKDIR /src

# Install build deps
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build the binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "-s -w" -o /basis-data-ltt ./

FROM alpine:latest
LABEL maintainer="Arie Brainware"

WORKDIR /app

# Set timezone to Jakarta
RUN apk add --no-cache tzdata && \
    cp /usr/share/zoneinfo/Asia/Jakarta /etc/localtime && \
    echo "Asia/Jakarta" > /etc/timezone && \
    apk del tzdata

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