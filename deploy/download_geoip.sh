#!/bin/sh
set -eu

# Host helper to download GeoIP DB and verify checksum for systemd or VM deployments.
# Expects environment variables:
#  GEOIP_URL or GEOIP_SHA256_URL or GEOIP_SHA256
#  GEOIP_DB_PATH (defaults to /etc/geoip/GeoLite2-Country.mmdb)

GEOIP_PATH=${GEOIP_DB_PATH:-/etc/geoip/GeoLite2-Country.mmdb}
GEOIP_DIR=$(dirname "$GEOIP_PATH")

# Support GitHub Releases download: set GITHUB_REPO (owner/repo) and GITHUB_RELEASE_TAG
# If GEOIP_URL is empty and those are provided, construct the releases download URL.
if [ -z "${GEOIP_URL:-}" ] && [ -n "${GITHUB_REPO:-}" ] && [ -n "${GITHUB_RELEASE_TAG:-}" ]; then
  GEOIP_URL="https://github.com/${GITHUB_REPO}/releases/download/${GITHUB_RELEASE_TAG}/$(basename "$GEOIP_PATH")"
  # If a token is provided, use it for authenticated downloads from private repos
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    AUTH_HEADER="Authorization: token ${GITHUB_TOKEN}"
  fi
fi

download_file() {
  url="$1"
  out="$2"
  n=0
  until [ "$n" -ge 5 ]
  do
    if command -v curl >/dev/null 2>&1; then
      if [ -n "${AUTH_HEADER:-}" ]; then
        curl -fSL --retry 3 --retry-delay 2 -H "$AUTH_HEADER" "$url" -o "$out" && return 0
      else
        curl -fSL --retry 3 --retry-delay 2 "$url" -o "$out" && return 0
      fi
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
  if [ -n "${GEOIP_SHA256:-}" ]; then
    case "$GEOIP_SHA256" in
      *' '* )
        echo "$GEOIP_SHA256" | sha256sum -c -
        return $? ;;
      *)
        actual=$(sha256sum "$file" | awk '{print $1}')
        [ "$actual" = "$GEOIP_SHA256" ]
        return $?
        ;;
    esac
  fi

  if [ -n "${GEOIP_SHA256_URL:-}" ]; then
    tmpsum="${file}.sha256.tmp"
    if ! download_file "$GEOIP_SHA256_URL" "$tmpsum"; then
      return 2
    fi
    (cd "$(dirname "$file")" && sha256sum -c "$(basename "$tmpsum")")
    return $?
  fi

  echo "No checksum specified" >&2
  return 3
}

main() {
  if [ -z "${GEOIP_URL:-}" ]; then
    echo "GEOIP_URL not set — skipping download" >&2
    return 0
  fi

  mkdir -p "$GEOIP_DIR"
  tmpfile="$GEOIP_PATH.download"

  echo "Downloading GeoIP DB from $GEOIP_URL"
  if ! download_file "$GEOIP_URL" "$tmpfile"; then
    echo "Failed to download GeoIP DB" >&2
    return 1
  fi

  echo "Verifying checksum..."
  if ! verify_checksum "$tmpfile"; then
    echo "Checksum verification failed" >&2
    rm -f "$tmpfile"
    return 2
  fi

  mv "$tmpfile" "$GEOIP_PATH"
  chown root:root "$GEOIP_PATH" || true
  chmod 0640 "$GEOIP_PATH" || true
  echo "GeoIP DB installed at $GEOIP_PATH"
}

main
