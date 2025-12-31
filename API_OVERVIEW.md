# LTT Backend API Overview

## API Base Information

- **Base URL**: `http://localhost:19091`
- **Swagger Documentation**: `http://localhost:19091/swagger/index.html`
- **API Version**: 1.0
- **License**: MIT

## Authentication

The API uses two types of authentication:

1. **Bearer Token Authentication** (`Authorization` header)
   - Format: `Authorization: Bearer <JWT_token>`
   - Used for initial authentication

2. **Session Token Authentication** (`session-token` header)
   - Format: `session-token: <session_token>`
   - Used for authenticated requests after login

## Endpoint Summary

### Authentication (Public)
| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| POST | `/login` | User login | No |
| POST | `/signup` | User registration | No |
| GET | `/token/validate` | Validate session token | Yes (Session) |
| DELETE | `/logout` | User logout | Yes (Session) |

### Patient Management
| Method | Endpoint | Description | Auth Required | Role |
|--------|----------|-------------|---------------|------|
| POST | `/patient` | Create patient (public) | No | - |
| GET | `/patient` | List patients | Yes | Admin |
| GET | `/patient/{id}` | Get patient details | Yes | Admin |
| PATCH | `/patient/{id}` | Update patient | Yes | Admin |
| DELETE | `/patient/{id}` | Delete patient | Yes | Admin |

### Disease Management
| Method | Endpoint | Description | Auth Required | Role |
|--------|----------|-------------|---------------|------|
| GET | `/disease` | List diseases | Yes | Admin |
| POST | `/disease` | Create disease | Yes | Admin |
| GET | `/disease/{id}` | Get disease details | Yes | Admin |
| PATCH | `/disease/{id}` | Update disease | Yes | Admin |
| DELETE | `/disease/{id}` | Delete disease | Yes | Admin |

### Treatment Management
| Method | Endpoint | Description | Auth Required | Role |
|--------|----------|-------------|---------------|------|
| GET | `/treatment` | List treatments | Yes | Admin, Therapist |
| POST | `/treatment` | Create treatment | Yes | Admin, Therapist |
| PATCH | `/treatment/{id}` | Update treatment | Yes | Admin, Therapist |
| DELETE | `/treatment/{id}` | Delete treatment | Yes | Admin, Therapist |

### Therapist Management
| Method | Endpoint | Description | Auth Required | Role |
|--------|----------|-------------|---------------|------|
| GET | `/therapist` | List therapists | Yes | Admin |
| POST | `/therapist` | Create therapist | Yes | Admin |
| GET | `/therapist/{id}` | Get therapist details | Yes | Admin |
| PATCH | `/therapist/{id}` | Update therapist | Yes | Admin |
| PUT | `/therapist/{id}` | Approve therapist | Yes | Admin |
| DELETE | `/therapist/{id}` | Delete therapist | Yes | Admin |

## User Roles

1. **Admin** (role_id: 1)
   - Full access to all endpoints
   - Can manage patients, diseases, treatments, and therapists

2. **Patient** (role_id: 2)
   - Limited access (not documented in current API)

3. **Therapist** (role_id: 3)
   - Can manage treatments
   - Can view patients (through treatment context)

## Response Format

All API responses follow a standard format:

### Success Response
```json
{
  "success": true,
  "error": "",
  "msg": "Operation successful",
  "data": { ... }
}
```

### Error Response
```json
{
  "success": false,
  "error": "Error description",
  "msg": "User-friendly message",
  "data": {}
}
```

## Common Query Parameters

### Pagination
- `limit` - Number of results to return (integer)
- `offset` - Number of results to skip (integer)

### Filtering
- `keyword` - Search keyword for text fields (string)
- `group_by_date` - Filter by date range (string)
  - Options: `last_2_days`, `last_3_months`, `last_6_months`

### Treatment-specific
- `therapist_id` - Filter by therapist ID (integer)
- `filter_by_therapist` - Filter by logged-in therapist (boolean)

## Sample Request/Response

### Login Request
```bash
curl -X POST http://localhost:19091/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'
```

### Login Response
```json
{
  "success": true,
  "error": "",
  "msg": "Login successful",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "role": "Admin",
    "user_id": 1
  }
}
```

### Authenticated Request
```bash
curl -X GET http://localhost:19091/patient \
  -H "Authorization: Bearer <jwt_token>" \
  -H "session-token: <session_token>"
```

## Getting Started

1. **Start the server**
   ```bash
   go run main.go
   ```

2. **Access Swagger UI**
   - Open browser: `http://localhost:19091/swagger/index.html`

3. **Test Authentication**
   - Use the `/signup` endpoint to create an account
   - Use the `/login` endpoint to get tokens
   - Click "Authorize" in Swagger UI and enter tokens

4. **Explore Endpoints**
   - Try out endpoints directly from Swagger UI
   - View request/response schemas
   - Test with sample data

## Development Notes

### Regenerating Documentation

When API endpoints are modified:

```bash
# Install swag CLI (first time only)
go install github.com/swaggo/swag/cmd/swag@latest

# Generate documentation
swag init --parseDependency --parseInternal
```

### Adding New Endpoints

1. Add Swagger annotations to handler function
2. Run `swag init` to regenerate docs
3. Restart server
4. Check Swagger UI for new endpoint

### Example Annotation
```go
// GetExample godoc
// @Summary      Example endpoint
// @Description  Detailed description
// @Tags         Example
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Item ID"
// @Success      200 {object} util.APIResponse
// @Router       /example/{id} [get]
func GetExample(c *gin.Context) {
    // handler code
}
```
