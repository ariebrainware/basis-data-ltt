Local TLS troubleshooting and trust instructions

If you see errors like:

```
http: TLS handshake error from [::1]:36750: remote error: tls: unknown certificate
```

the client (browser or curl) is refusing the server's self-signed certificate because it is not trusted.

Options to resolve this for local development (Linux):

- Trust the generated cert system-wide (Debian/Ubuntu):

```bash
sudo cp certs/localhost.crt /usr/local/share/ca-certificates/localhost.crt
sudo update-ca-certificates
# Restart browser (and your app) to pick up new trust store
```

- Trust the cert on Fedora/CentOS/RHEL:

```bash
sudo cp certs/localhost.crt /etc/pki/ca-trust/source/anchors/
sudo update-ca-trust extract
```

- Firefox (if it uses its own store):
  - Open Preferences → Privacy & Security → View Certificates → Authorities → Import
  - Select `certs/localhost.crt` and enable trust for websites

- Use mkcert to create locally-trusted certificates (recommended):

```bash
# install mkcert (example for Debian/Ubuntu)
sudo apt install libnss3-tools
curl -JLO "https://github.com/FiloSottile/mkcert/releases/latest/download/mkcert-$(uname -m)-linux"
chmod +x mkcert-$(uname -m)-linux && sudo mv mkcert-$(uname -m)-linux /usr/local/bin/mkcert

# create a local CA and generate certs for localhost and loopback IPs
mkcert -install
mkcert localhost 127.0.0.1 ::1

# mkcert generates files like: localhost+2.pem (cert) and localhost+2-key.pem (key)
# Point your .env TLS_CERT_FILE/TLS_KEY_FILE to those files or copy them to `certs/`
```

- Temporary: skip verification for testing with `curl`:

```bash
curl -k https://localhost:19091/
```

Notes:
- After trusting the cert or installing an mkcert CA, restart your browser and the Go server.
- The `scripts/generate_local_tls.sh` creates `certs/localhost.crt` and `certs/localhost.key`. Trust the `.crt` file.
- Be careful: adding CAs to the system store grants them trust for all apps — only trust files you created locally.
