# basis-data-ltt

Lightweight Go REST API for managing patients, diseases, treatments, therapists and sessions.

---

## Table of Contents

- [Quick links](#quick-links)
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

```
APPENV=local        # local|development|production|test
APPPORT=19091
APITOKEN=<api-token-for-cors-middleware>
JWTSECRET=<jwt-secret-used-for-signing-and-password-hmac>
DBHOST=127.0.0.1
DBPORT=3306
DBNAME=basis_data_ltt
DBUSER=root
DBPASS=password
GINMODE=debug
```

Note: The application stores the JWT secret in memory via `util.SetJWTSecret()` on startup. Tests set `APPENV=test` and use an in-memory SQLite DB.

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
- Passwords should be hashed using a dedicated slow password hashing function (for example, bcrypt or Argon2 with per-user salts). Do not reuse `JWTSECRET` (or any JWT signing key) for password hashing; see [util/password.go](util/password.go) and update it if needed to follow this guidance.
- Session tokens are stored in the `sessions` table and cached in Redis when available (see [endpoint/authentication.go](endpoint/authentication.go)).

If you'd like, I can also add a quick `make` target or Docker instructions to simplify local setup.

