# Copilot Instructions for Basis Data LTT Backend

## Project Overview
A Go REST API backend for managing patient data, treatments, and therapy sessions using Gin framework and MySQL with GORM ORM. Built for Jakarta timezone with JWT-based authentication and role-based access control.

## Architecture & Key Components

### Layered Structure
- **main.go**: Entry point that initializes config, database, routes, and server
- **config/**: Singleton pattern for loading environment variables and establishing MySQL connections
- **endpoint/**: HTTP handlers organized by domain (authentication, patient, disease, treatment, therapist)
- **model/**: GORM models with auto-migration support (Patient, User, Disease, Treatment, Session, Therapist, Role, PatientCode)
- **middleware/**: CORS configuration and JWT token validation
- **util/**: Helper functions for password hashing, error handling, and API responses

### Data Flow
1. **Request** → CORS middleware validates API token → Route handler in endpoint/
2. **Handler** → Loads DB connection from config singleton → Uses model structs with GORM queries
3. **Response** → Standardized via `util.CallSuccessOK()` and util error helper functions

### Critical Design Decisions
- **Singleton Config**: `config.LoadConfig()` uses `sync.Once` for thread-safe initialization (see [config/config.go](../config/config.go))
- **Middleware Authentication**: Token validation via `Authorization: Bearer <token>` header, except OPTIONS preflight requests
- **Auto-migration**: Models are auto-migrated on startup; add new models to migration list in [main.go](../main.go)
- **Timezone**: Hardcoded Asia/Jakarta; TLS timezone data embedded in binary

## Developer Workflows

### Build & Run
```bash
# Local development (requires .env file)
go run main.go

# Build binary
go build -o ltt-be

# Run Docker container
docker build -t ltt-be . && docker run -p 19091:19091 ltt-be
```

### Database
- MySQL required; connection via DSN: `user:pass@tcp(host:port)/dbname`
- Models auto-migrate on startup
- Seed data: `model.SeedRoles(db)` called in main.go

### Environment Variables
Key variables (see README.md for full list):
- `APPENV`: local|production (controls logging verbosity)
- `APPPORT`: Server port (default 19091)
- `APITOKEN`: Bearer token for API validation
- `DBHOST`, `DBPORT`, `DBNAME`, `DBUSER`, `DBPASS`: MySQL connection

## Code Patterns & Conventions

### Endpoint Handlers
All endpoints follow this pattern (see [endpoint/authentication.go](../endpoint/authentication.go)):
1. Bind JSON request: `c.ShouldBindJSON(&req)`
2. Connect DB: `config.ConnectMySQL()`
3. Execute query with GORM
4. Return via util helpers: `util.CallSuccessOK()` or `util.CallUserError()`

### Model Structs
- Embed `gorm.Model` for automatic ID, CreatedAt, UpdatedAt, DeletedAt
- Use struct tags for JSON serialization and GORM column mapping
- Example: [model/patient.go](../model/patient.go) - FullName mapped to full_name column

### Error Handling
Use util package error functions (see [util/helperfunc.go](../util/helperfunc.go)):
- `util.CallUserError()`: 400 Bad Request (validation/user errors)
- `util.CallServerError()`: 500 Internal Server Error
- `util.CallErrorNotFound()`: 404 Not Found

All errors wrapped in standardized `APIResponse` struct with Success, Error, Msg, Data fields.

## Route Structure & Access Control

### Public Routes
- `POST /patient` - Patient self-registration (no auth required)
- `POST /login` - User login
- `POST /signup` - User sign-up
- `GET /token/validate` - Token validation

### Protected Routes (require `ValidateLoginToken()` middleware)
```
/patient                  - CRUD operations
  /treatment              - Nested treatment management
/disease                  - CRUD operations
/therapist                - CRUD operations with approval endpoint
/logout                   - Session termination
```

See [main.go](../main.go) for complete routing configuration.

## Cross-Component Communication

### Endpoints → Models
Endpoints import and use model structs directly with GORM queries. No service layer abstraction—business logic resides in endpoint handlers.

### Middleware → Models
JWT validation middleware ([middleware/middleware.go](../middleware/middleware.go)) verifies tokens but does not load user context into request (add if needed).

## Testing & Debugging
- No automated tests currently exist in the repository
- Gin runs in configured mode (debug/release) based on `GINMODE` env var
- SQL query logging enabled in local/debug mode; disabled in production
- Slow query threshold: 200ms (see [config/config.go](../config/config.go#L76))
- Test API endpoints manually using tools like curl, Postman, or similar

## Key Files Reference
| File | Purpose |
|------|---------|
| [main.go](../main.go) | Route setup, migrations, server initialization |
| [config/config.go](../config/config.go) | Singleton config & MySQL connection |
| [middleware/middleware.go](../middleware/middleware.go) | CORS + JWT token validation |
| [endpoint/authentication.go](../endpoint/authentication.go) | Login, signup, token handling |
| [model/patient.go](../model/patient.go) | Patient entity with GORM tags |
| [util/helperfunc.go](../util/helperfunc.go) | Standardized error/success responses |

## Common Tasks

### Adding a New Entity
1. Create model struct in [model/](../model/) with `gorm.Model` and JSON tags
2. Add to auto-migration list in [main.go](../main.go)
3. Create endpoint handlers in [endpoint/](../endpoint/) following existing patterns
4. Register routes in main.go route groups
5. Use util error functions for responses

### Modifying Authentication
JWT implementation in [endpoint/token.go](../endpoint/token.go); middleware validation in [middleware/middleware.go](../middleware/middleware.go). Token expiry and claims logic centralized in token.go.

### Database Schema Changes
Models control schema via GORM tags. Update struct → update migration list → restart server. Use `gorm.Model` for standard audit fields.
