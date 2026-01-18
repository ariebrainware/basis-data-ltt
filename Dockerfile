FROM alpine:latest
LABEL maintainer="Arie Brainware"
WORKDIR /app

# Declare build args (optional at image build time)
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
    JWTSECRET=$JWTSECRET \
    REDIS_HOST=$REDIS_HOST \
    REDIS_PORT=$REDIS_PORT

# Set timezone to Jakarta
RUN apk add --no-cache tzdata && \
    cp /usr/share/zoneinfo/Asia/Jakarta /etc/localtime && \
    echo "Asia/Jakarta" > /etc/timezone && \
    apk del tzdata

# Copy the prebuilt binary produced by goreleaser into the image.
# Goreleaser provides the binary in the Docker build context as 'basis-data-ltt'.
COPY basis-data-ltt ./basis-data-ltt
RUN chmod +x ./basis-data-ltt

EXPOSE 19091

CMD ["./basis-data-ltt"]