#!/usr/bin/env bash
set -euo pipefail

# Generates a self-signed certificate for localhost with SANs for 127.0.0.1 and ::1
# Output files: certs/localhost.crt and certs/localhost.key

OUT_DIR="certs"
CRT_PATH="$OUT_DIR/localhost.crt"
KEY_PATH="$OUT_DIR/localhost.key"
CONF_PATH="$OUT_DIR/localhost.cnf"

mkdir -p "$OUT_DIR"

cat > "$CONF_PATH" <<EOF
[ req ]
default_bits       = 2048
distinguished_name = req_distinguished_name
req_extensions     = req_ext
prompt             = no

[ req_distinguished_name ]
CN = localhost

[ req_ext ]
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = localhost
IP.1 = 127.0.0.1
IP.2 = ::1

[ v3_req ]
subjectAltName = @alt_names
EOF

echo "Generating self-signed certificate for localhost..."
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout "$KEY_PATH" -out "$CRT_PATH" \
  -config "$CONF_PATH" -extensions v3_req

chmod 600 "$KEY_PATH" || true

echo "Certificate generated:" 
echo "  cert: $CRT_PATH"
echo "  key : $KEY_PATH"
echo "To run the server with TLS, ensure .env TLS_CERT_FILE/TLS_KEY_FILE point to these files."

exit 0
