#!/bin/sh
set -eu

# CI helper: upload GeoIP DB and checksum to S3 and print presigned URLs.
# Requires: AWS CLI configured with permissions to put objects and generate presign.

BUCKET=${BUCKET:-my-secure-bucket}
KEY_PREFIX=${KEY_PREFIX:-geoip}
FILE=${1:-data/GeoLite2-Country.mmdb}
# Optional: provider can be 's3' or 'github'. If not set, script will prefer S3 unless
# `GITHUB_REPO` and `GH_TOKEN`/`GITHUB_TOKEN` are provided and `PROVIDER=github`.
PROVIDER=${PROVIDER:-}

# GitHub variables (for provider=github)
GH_REPO=${GH_REPO:-$GITHUB_REPO}
GH_TOKEN=${GH_TOKEN:-$GITHUB_TOKEN}


if [ ! -f "$FILE" ]; then
  echo "file not found: $FILE" >&2
  exit 2
fi

sha256sum "$FILE" > "$FILE.sha256"

if [ -z "$PROVIDER" ] && [ -n "$GH_REPO" ] && [ -n "$GH_TOKEN" ]; then
  PROVIDER=github
fi

case "$PROVIDER" in
  github)
    # Use GitHub CLI if available, otherwise require it for simplicity
    if command -v gh >/dev/null 2>&1; then
      TAG=${TAG:-geoip-$(date +%Y%m%d%H%M%S)}
      echo "Creating release $TAG on $GH_REPO and uploading assets"
      # Ensure GH auth; gh uses GH_TOKEN/GITHUB_TOKEN env or local auth
      gh repo set-default "$GH_REPO" >/dev/null 2>&1 || true
      gh release create "$TAG" --title "$TAG" --notes "GeoIP DB" --draft || true
      gh release upload "$TAG" "$FILE" "$FILE.sha256" --clobber
      gh release edit "$TAG" --draft=false || true
      echo "Uploaded to GitHub Releases: https://github.com/$GH_REPO/releases/tag/$TAG"
      echo "Direct asset URL: https://github.com/$GH_REPO/releases/download/$TAG/$(basename "$FILE")"
    else
      echo "gh CLI not found. Install GitHub CLI or set PROVIDER=s3." >&2
      exit 3
    fi
    ;;
  s3|S3)
    aws s3 cp "$FILE" "s3://$BUCKET/$KEY_PREFIX/$(basename "$FILE")"
    aws s3 cp "$FILE.sha256" "s3://$BUCKET/$KEY_PREFIX/$(basename "$FILE.sha256")"

    echo "Uploaded to s3://$BUCKET/$KEY_PREFIX/"
    echo "Presigned URLs (1 hour):"
    aws s3 presign "s3://$BUCKET/$KEY_PREFIX/$(basename "$FILE")" --expires-in 3600
    aws s3 presign "s3://$BUCKET/$KEY_PREFIX/$(basename "$FILE.sha256")" --expires-in 3600
    ;;
  *)
    echo "Unknown provider: ${PROVIDER:-<not set>}" >&2
    exit 4
    ;;
esac
