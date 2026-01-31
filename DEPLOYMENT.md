Production deployment notes
==========================

This repository includes helpers for deploying the GeoIP DB securely in production (VM or container hosts).

Options provided:

- Container runtime: `entrypoint.sh` (already added) downloads `GEOIP_URL` and verifies `GEOIP_SHA256` or `GEOIP_SHA256_URL` before starting the binary.
- VM / systemd: `deploy/download_geoip.sh` is a host script; `deploy/ltt.service` is a systemd unit template invoking that script before starting the binary.
- CI helper: `scripts/ci_upload_geoip.sh` uploads `data/GeoLite2-Country.mmdb` and checksum to S3 and prints presigned URLs.

Quick docker run example (for a docker-host production without compose):

```bash
# Environment variables should be provided via your secret manager or CI — avoid committing them into files.
export GEOIP_URL="https://...presigned-url..."
export GEOIP_SHA256="<hex>"

docker run --rm \
  -e GEOIP_URL="$GEOIP_URL" \
  -e GEOIP_SHA256="$GEOIP_SHA256" \
  -v geoip-data:/etc/geoip \
  --name ltt \
  myregistry/basis-data-ltt:latest
```

GitHub Releases (alternative to S3)
----------------------------------

You can use GitHub Releases to store the `.mmdb` and `.sha256` as release assets. This works for public or private repos (private requires auth).

CI upload (example using `gh` CLI):

```bash
# from CI where GH_TOKEN/GITHUB_TOKEN is available in secrets
FILE=data/GeoLite2-Country.mmdb
sha256sum "$FILE" > "$FILE.sha256"
TAG=v-geoip-$(date +%Y%m%d%H%M%S)
gh release create "$TAG" --title "$TAG" --notes "GeoIP DB" --draft
gh release upload "$TAG" "$FILE" "$FILE.sha256" --clobber
gh release edit "$TAG" --draft=false
echo "https://github.com/${GH_REPO:-owner/repo}/releases/download/$TAG/$(basename $FILE)"
```

Host download (systemd / VM):

Set environment variables securely (e.g. `/etc/default/ltt`):

```
GITHUB_REPO=owner/repo
GITHUB_RELEASE_TAG=v-geoip-20260130010101
GITHUB_TOKEN="<short-lived-token>"
GEOIP_DB_PATH=/etc/geoip/GeoLite2-Country.mmdb
```

The `deploy/download_geoip.sh` script will construct the releases URL when `GITHUB_REPO` and `GITHUB_RELEASE_TAG` are present and will use `GITHUB_TOKEN` for authenticated downloads if provided.

Notes:
- Prefer short-lived tokens (rotate often) or have your deployment CI fetch the asset and place it on the target host.
- Releases are versioned and easy to audit, but not a substitute for a secret manager for production secrets.


Systemd installation steps (VM)

1. Copy binary and script to system paths (run as root during install):

```bash
install -m 0755 -o root -g root ./basis-data-ltt /usr/local/bin/basis-data-ltt
install -m 0755 -o root -g root ./deploy/download_geoip.sh /usr/local/bin/download_geoip.sh
mkdir -p /etc/geoip
```

2. Create a drop-in env file `/etc/default/ltt` containing secrets (use your secret manager to write this):

```
# /etc/default/ltt
GEOIP_URL="https://...presigned-url..."
GEOIP_SHA256_URL="https://...presigned-url-to-sha256..."
GEOIP_DB_PATH="/etc/geoip/GeoLite2-Country.mmdb"
```

3. Install systemd unit and enable service:

```bash
install -m 0644 -o root -g root ./deploy/ltt.service /etc/systemd/system/ltt.service
systemctl daemon-reload
systemctl enable --now ltt.service
```

Notes & security
- Use short-lived presigned URLs or instance role-based access; do not keep long-lived keys inside the env file.
- Enable server-side encryption and bucket access logging for the object store.
- Verify checksum on every install (scripts enforce this).
