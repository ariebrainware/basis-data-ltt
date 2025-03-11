# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Cache go modules installation
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build the binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o app .

# Run stage
FROM alpine:latest
LABEL maintainer="Arie Brainware"
WORKDIR /app

# Declare build args
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

# Optionally, set them as environment variables inside the image
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
    JWTSECRET=$JWTSECRET
    
# Set timezone to UTC+7.
RUN apk add --no-cache tzdata && \
    cp /usr/share/zoneinfo/Asia/Bangkok /etc/localtime && \
    echo "Asia/Jakarta" > /etc/timezone && \
    apk del tzdata

# Copy binary from builder stage.
COPY --from=builder /app/app .

# Expose port if needed (e.g., 8080)
EXPOSE 19091

CMD ["./app"]