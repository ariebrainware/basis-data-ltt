#!/bin/sh
set -eu

# Entrypoint: fetch GeoIP DB if `GEOIP_URL` is provided, verify checksum, set permissions, then exec the app

GEOIP_PATH=${GEOIP_DB_PATH:-/etc/geoip/GeoLite2-Country.mmdb}
GEOIP_DIR=$(dirname "$GEOIP_PATH")

download_file() {
  url="$1"
  out="$2"
  # retry a few times
  n=0
  until [ "$n" -ge 5 ]
  do
    if command -v curl >/dev/null 2>&1; then
      curl -fSL --retry 3 --retry-delay 2 "$url" -o "$out" && return 0
    else
      wget -O "$out" "$url" && return 0
    fi
    n=$((n+1))
    sleep 1
  done
  return 1
}

verify_checksum() {
  file="$1"
  # If GEOIP_SHA256 contains full "<hex>  <filename>" then use sha256sum -c
  if [ -n "${GEOIP_SHA256:-}" ]; then
    case "$GEOIP_SHA256" in
      *' '* )
        echo "$GEOIP_SHA256" | sha256sum -c -
        return $? ;;
      *)
        # Only hash provided — compute and compare
        actual=$(sha256sum "$file" | awk '{print $1}')
        if [ "$actual" = "$GEOIP_SHA256" ]; then
          return 0
        else
          return 2
        fi
        ;;
    esac
  fi

  # If SHA256 URL provided, download and verify
  if [ -n "${GEOIP_SHA256_URL:-}" ]; then
    tmpsum="${file}.sha256.tmp"
    if ! download_file "$GEOIP_SHA256_URL" "$tmpsum"; then
      return 3
    fi
    # Ensure file references match or just use sha256sum -c
    (cd "$(dirname "$file")" && sha256sum -c "$(basename "$tmpsum")")
    return $?
  fi

  # No checksum provided — fail by default
  echo "No checksum provided (GEOIP_SHA256 or GEOIP_SHA256_URL)" >&2
  return 4
}

if [ -n "${GEOIP_URL:-}" ]; then
  mkdir -p "$GEOIP_DIR"
  tmpfile="$GEOIP_PATH.download"
  echo "Downloading GeoIP DB from ${GEOIP_URL} to ${GEOIP_PATH}"
  if ! download_file "$GEOIP_URL" "$tmpfile"; then
    echo "Failed to download GeoIP DB" >&2
    exit 1
  fi

  echo "Verifying checksum..."
  if ! verify_checksum "$tmpfile"; then
    echo "Checksum verification failed" >&2
    rm -f "$tmpfile"
    exit 1
  fi

  mv "$tmpfile" "$GEOIP_PATH"
  chown 1000:1000 "$GEOIP_PATH" || true
  chmod 0640 "$GEOIP_PATH" || true
  echo "GeoIP DB ready at $GEOIP_PATH"
fi

exec "$@"
