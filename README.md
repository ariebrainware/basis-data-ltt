# basis-data-ltt

Lightweight Go REST API for managing patients, diseases, treatments, therapists and sessions with comprehensive security features.

---

## Table of Contents

- [Quick links](#quick-links)
- [Security](#security)
- [Prerequisites](#prerequisites)
- [Setup](#setup)
- [Build & Run](#build--run)
- [API Documentation](#api-documentation)
- [Important Routes](#important-routes)
- [Testing](#testing)
- [Notes for Contributors](#notes-for-contributors)

---

## Quick links

- Code: [main.go](main.go)
- Configuration: [config/config.go](config/config.go)
- Middleware: [middleware/middleware.go](middleware/middleware.go)
- Authentication & endpoints: [endpoint/authentication.go](endpoint/authentication.go)
- Models: [model](model)
- Utility helpers: [util/helperfunc.go](util/helperfunc.go) and [util/password.go](util/password.go)
- Swagger docs entrypoint: [docs/swagger.yaml](docs/swagger.yaml)
- **Security Guide**: [SECURITY.md](SECURITY.md)

---

## Security

This application implements comprehensive security features:

- **Argon2id Password Hashing** - Industry-standard password hashing with unique salts
- **Rate Limiting** - Protection against brute force attacks (5 attempts per 15 minutes)
- **Account Lockout** - Automatic lockout after 5 failed login attempts
- **Security Logging** - Comprehensive audit trail of security events
- **HTTPS/TLS Support** - Encrypted communication support
- **HSTS Headers** - HTTP Strict Transport Security
- **SQL Injection Prevention** - Parameterized queries via GORM
- **Input Validation** - Request validation on all endpoints
- **Session Management** - Secure token-based authentication with 1-hour expiration

For detailed security information, configuration, and best practices, see [SECURITY.md](SECURITY.md).

---

## Prerequisites

- Go 1.24.0+
- (Optional) MySQL for local development; tests use an in-memory SQLite when `APPENV=test`.

## Setup

1. Clone and enter the repo:

```bash
git clone https://github.com/ariebrainware/basis-data-ltt.git
cd basis-data-ltt
```

2. Download dependencies:

```bash
go mod download
```

3. Copy or create a `.env` file for local development. Important environment variables:

```bash
# Application Configuration
APPENV=local        # local|development|production|test
APPPORT=19091
APITOKEN=<api-token-for-cors-middleware>
JWTSECRET=<jwt-secret-used-for-signing> # Use a strong secret (min 32 chars)
GINMODE=debug

# Database Configuration
DBHOST=127.0.0.1
DBPORT=3306
DBNAME=basis_data_ltt
DBUSER=root
DBPASS=password

# Redis Configuration (optional, for rate limiting and caching)
REDIS_ADDR=localhost:6379
REDIS_PASS=
REDIS_DB=0

# TLS/HTTPS Configuration (optional, for production)
ENABLE_TLS=false
TLS_CERT_FILE=/path/to/cert.pem
TLS_KEY_FILE=/path/to/key.pem

# HSTS Configuration (optional, recommended for production with TLS)
ENABLE_HSTS=false
HSTS_MAX_AGE=31536000
HSTS_INCLUDE_SUBDOMAINS=true
```

See [.env.sample](.env.sample) for all available configuration options.

**Security Notes:**
- Use a strong `JWTSECRET` (minimum 32 random characters)
- Never commit `.env` files to version control
- In production, use environment variables instead of `.env` files
- See [SECURITY.md](SECURITY.md) for security best practices

## Build & Run

Build:

```bash
go build -o basis-data-ltt
```

Run (development):

```bash
go run main.go
```

Server defaults to `:APPPORT` (19091). The app sets timezone to `Asia/Jakarta` on startup.

---

## API Documentation

Run the server and open:

```
http://localhost:19091/swagger/index.html
```

API docs are generated with `swag` from code annotations. To regenerate docs locally:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init --parseDependency --parseInternal
```

---

## Important Routes

Authentication:
- `POST /signup` - register
- `POST /login` - obtain session token
- `DELETE /logout` - invalidate session (requires `session-token` header)
- `GET /token/validate` - validate session token
- `POST /verify-password` - (protected) verify current user's password before allowing password change

Patient (admin):
- `POST /patient` - create patient (public)
- `GET|PATCH|DELETE /patient/:id` - manage patients (admin)

Disease (admin):
- `GET|POST|PATCH|DELETE /disease`

Treatment (admin, therapist):
- `GET|POST|PATCH|DELETE /treatment`

Therapist (admin):
- `GET|POST|PATCH|PUT|DELETE /therapist`

See the Swagger UI for full request/response schemas.

---

## Testing

Unit and integration tests are included. Tests set `APPENV=test` and use an in-memory SQLite DB. Run:

```bash
go test ./...
```

If a test needs to run against MySQL, set environment variables accordingly. Most CI/test code in this repo uses the in-memory DB when `APPENV=test`.

---

## Notes for Contributors

- The config loader is a singleton: see [config/config.go](config/config.go).
- Database connection is injected into Gin context via `middleware.DatabaseMiddleware` ([middleware/middleware.go](middleware/middleware.go)).
- **Passwords are hashed using Argon2id** with unique per-user salts. The implementation is in [util/password.go](util/password.go). Never use the JWT secret for password hashing.
- Session tokens are stored in the `sessions` table and cached in Redis when available (see [endpoint/authentication.go](endpoint/authentication.go)).
- Rate limiting is implemented using Redis when available; see [middleware/ratelimit.go](middleware/ratelimit.go).
- **Security logging** is enabled for all authentication and authorization events; see [util/security_logger.go](util/security_logger.go).
- Review [SECURITY.md](SECURITY.md) before making changes to authentication, authorization, or password handling code.

If you'd like, I can also add a quick `make` target or Docker instructions to simplify local setup.

## GeoIP Local Database (optional)

This project can use a local MaxMind GeoIP2/GeoLite2 `.mmdb` file to resolve IPs to a city and country for `SecurityLog` entries.

- Place your `.mmdb` file somewhere accessible and set the environment variable `GEOIP_DB_PATH` to its path.
- The application will initialize the GeoIP reader on startup when `GEOIP_DB_PATH` is set. You can also call `util.DownloadGeoIP()` programmatically to download a file and `util.ValidateGeoIP()` to validate it.
- The code includes an in-memory cache with 24h TTL to avoid repeated lookups. Metrics are available via `util.GetGeoIPCacheMetrics()` (cache hits, misses, size).

Example usage (manual):

```bash
# export GEOIP_DB_PATH=/opt/geoip/GeoLite2-City.mmdb
export GEOIP_DB_PATH=/path/to/GeoLite2-City.mmdb
go run main.go
```

If you need help automating downloading the GeoIP DB (MaxMind requires agreeing to their license), I can add a small script that downloads and validates the DB given a signed URL or local mirror.


