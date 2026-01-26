# Security Guide

This document describes the security features implemented in the LTT Backend API and how to configure them.

## Overview

The backend implements multiple layers of security to protect user data and prevent unauthorized access:

1. **Strong Password Hashing** - Argon2id with unique salts
2. **Rate Limiting** - Protection against brute force attacks
3. **Account Lockout** - Automatic account locking after failed attempts
4. **Security Logging** - Comprehensive audit trail of security events
5. **HTTPS/TLS Support** - Encrypted communication
6. **HSTS Headers** - HTTP Strict Transport Security
7. **SQL Injection Prevention** - Parameterized queries via GORM
8. **Input Validation** - Request validation on all endpoints
9. **Session Management** - Secure token-based authentication with expiration

## Password Security

### Argon2id Hashing

The application uses Argon2id for password hashing, which is the winner of the Password Hashing Competition and recommended by OWASP.

**Parameters:**
- Algorithm: Argon2id (hybrid of Argon2i and Argon2d)
- Time cost: 3 iterations
- Memory cost: 64 MB
- Parallelism: 4 threads
- Key length: 32 bytes
- Salt length: 16 bytes (cryptographically secure random)

**Effective Cost:**
The effective cost parameter is approximately 16, which exceeds the minimum requirement of 12.

**Password Requirements:**
- Minimum length: 8 characters
- Unique salt generated per user
- Salt stored in database alongside hashed password

### Password Storage Format

New passwords are stored in the format:
```
argon2id$base64(salt)$base64(hash)
```

### Backward Compatibility

The system maintains backward compatibility with legacy HMAC-SHA256 passwords during migration. When a user with a legacy password logs in successfully, their password will be re-hashed using Argon2id on next password change.

## Rate Limiting

### Configuration

Rate limiting is automatically applied to authentication endpoints to prevent brute force attacks.

**Limits:**
- `/login` endpoint: 5 attempts per 15 minutes per IP address
- `/signup` endpoint: 5 attempts per 15 minutes per IP address

**Implementation:**
- Uses Redis for distributed rate limiting when available
- Gracefully degrades if Redis is unavailable (allows requests)
- Rate limit counters are per IP address and endpoint
- Counters automatically expire after the time window

**Response:**
When rate limit is exceeded:
```json
{
  "success": false,
  "error": "rate limit exceeded",
  "msg": "Too many requests. Please try again later.",
  "data": {}
}
```

## Account Lockout

### Failed Login Protection

Accounts are automatically locked after multiple failed login attempts to prevent brute force attacks.

**Configuration:**
- Failed attempts threshold: 5 attempts
- Lockout duration: 15 minutes
- Counter resets on successful login

**Behavior:**
1. Each failed login increments `failed_attempts` counter
2. After 5 failed attempts, account is locked until `locked_until` timestamp
3. Locked accounts cannot login even with correct password
4. Successful login resets the failed attempts counter
5. Lockout automatically expires after 15 minutes

**Response for Locked Account:**
```json
{
  "success": false,
  "error": "account locked",
  "msg": "Account is locked until 2026-01-26T15:30:00Z due to multiple failed login attempts",
  "data": null
}
```

## Security Logging

### Event Types

The application logs the following security events:

1. **LOGIN_SUCCESS** - Successful user authentication
2. **LOGIN_FAILURE** - Failed login attempt
3. **LOGOUT** - User logout
4. **ACCOUNT_LOCKED** - Account locked due to failed attempts
5. **PASSWORD_CHANGED** - Password change event
6. **UNAUTHORIZED_ACCESS** - Unauthorized access attempt
7. **RATE_LIMIT_EXCEEDED** - Rate limit threshold exceeded
8. **SUSPICIOUS_ACTIVITY** - Suspicious behavior detected

### Log Format

Security logs are written to stdout with the prefix `[SECURITY]` and include:

```
[SECURITY] <timestamp> Event=<type> UserID=<id> Email=<email> IP=<address> UserAgent=<agent> Message=<description>
```

**Example:**
```
[SECURITY] 2026/01/26 03:24:32 Event=LOGIN_SUCCESS UserID=123 Email=user@example.com IP=192.168.1.100 UserAgent=Mozilla/5.0... Message=User logged in successfully
```

### Production Deployment

In production, security logs should be:
- Redirected to a separate log file
- Monitored for suspicious patterns
- Retained for audit purposes
- Protected from unauthorized access

## HTTPS/TLS Configuration

### Enabling HTTPS

To enable HTTPS, configure the following environment variables:

```bash
# Enable TLS
ENABLE_TLS=true

# Path to TLS certificate file (PEM format)
TLS_CERT_FILE=/path/to/certificate.pem

# Path to TLS private key file (PEM format)
TLS_KEY_FILE=/path/to/private-key.pem
```

### Certificate Requirements

- **Format**: PEM-encoded X.509 certificate
- **Type**: Valid SSL/TLS certificate (not self-signed in production)
- **Key**: RSA or ECDSA private key in PEM format

### Obtaining Certificates

**Production:**
- Use Let's Encrypt (free, automated)
- Purchase from a Certificate Authority
- Use your cloud provider's certificate service

**Development:**
Generate self-signed certificates:
```bash
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
```

**Note**: Self-signed certificates should only be used for development/testing.

## HSTS (HTTP Strict Transport Security)

### Configuration

HSTS headers instruct browsers to only communicate with the server over HTTPS.

```bash
# Enable HSTS header
ENABLE_HSTS=true

# Max age in seconds (default: 31536000 = 1 year)
HSTS_MAX_AGE=31536000

# Include subdomains (default: true)
HSTS_INCLUDE_SUBDOMAINS=true
```

### When to Enable HSTS

- **Enable** when using valid HTTPS certificates
- **Disable** for development with self-signed certificates
- **Disable** if serving over HTTP only

### HSTS Header Format

When enabled, the server sends:
```
Strict-Transport-Security: max-age=31536000; includeSubDomains
```

### Important Notes

1. HSTS is only sent when:
   - `ENABLE_HSTS=true` is set, OR
   - The request is received over TLS/HTTPS
2. Once a browser receives HSTS header, it will refuse HTTP connections
3. Use caution when enabling in production
4. Consider HSTS preloading for maximum security

## SQL Injection Prevention

### Parameterized Queries

The application uses GORM ORM which automatically uses parameterized queries for all database operations.

**Safe Examples:**
```go
// GORM automatically parameterizes these queries
db.Where("email = ?", email).First(&user)
db.Where("email = ? AND password = ?", email, hash).First(&user)
```

**What to Avoid:**
```go
// NEVER use string concatenation for queries
db.Raw("SELECT * FROM users WHERE email = '" + email + "'")
```

### Code Review Checklist

When reviewing code changes:
- ✅ All database queries use GORM methods
- ✅ No raw SQL with string concatenation
- ✅ User input is passed as query parameters
- ✅ No `db.Exec()` with interpolated strings

## Input Validation

### Request Validation

All endpoints validate input using Gin's binding and validation:

```go
type LoginRequest struct {
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required"`
}

type SignupRequest struct {
    Name     string `json:"name" binding:"required"`
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required,min=8"`
}
```
```

### Validation Rules

1. **Email**: Must be valid email format, unique in database
2. **Password**: Minimum 8 characters on signup
3. **Required Fields**: Validated via `binding:"required"` tags
4. **Type Safety**: JSON binding ensures correct types

### Custom Validation

Add custom validation for business rules:

```go
if len(req.Password) < 8 {
    return errors.New("password must be at least 8 characters")
}
```

## Session Management

### Session Tokens

- Generated using JWT with HMAC-SHA256 signing
- Stored in database `sessions` table
- Cached in Redis for performance (when available)
- Automatically expire after 1 hour
- Invalidated on logout

### Token Validation

Sessions are validated on every protected endpoint request:

1. Check Redis cache for fast validation
2. Fallback to database if Redis unavailable
3. Verify token expiration
4. Verify user exists and is not deleted
5. Load user ID and role into request context

### Session Security

- Session tokens should be transmitted via `session-token` header
- Never log or expose session tokens
- Tokens are unique per login
- Old tokens are invalidated on logout

## Security Best Practices

### Deployment Checklist

- [ ] Enable HTTPS/TLS with valid certificates
- [ ] Configure HSTS with appropriate max-age
- [ ] Set up Redis for rate limiting
- [ ] Configure security log monitoring
- [ ] Use strong `JWTSECRET` (minimum 32 characters)
- [ ] Set up firewall rules for database access
- [ ] Enable CORS with specific origins (not *)
- [ ] Review and restrict API token usage
- [ ] Set up automated security updates
- [ ] Configure backup and disaster recovery

### Environment Security

1. **Secrets Management:**
   - Never commit `.env` files to git
   - Use environment variables in production
   - Consider using secrets management (AWS Secrets Manager, Vault)
   - Rotate secrets regularly

2. **Database Security:**
   - Use strong database passwords
   - Limit database network access
   - Enable database encryption at rest
   - Regular database backups
   - Separate read replicas from write masters

3. **Application Security:**
   - Keep dependencies up to date
   - Run security scanners regularly
   - Monitor security advisories
   - Implement least privilege access
   - Regular security audits

### Monitoring and Alerting

Set up monitoring for:
- Failed login attempts (spike detection)
- Account lockouts
- Rate limit violations
- Unauthorized access attempts
- Unusual access patterns
- API error rates

## Incident Response

### Account Compromise

If an account is compromised:

1. Lock the account immediately
2. Invalidate all active sessions
3. Force password reset
4. Review security logs
5. Notify the user
6. Investigate root cause

### Security Breach

In case of a security breach:

1. Contain the incident
2. Preserve evidence
3. Assess the impact
4. Notify affected users
5. Fix the vulnerability
6. Document lessons learned

## Security Contact

For security concerns or to report vulnerabilities:
- Email: support@ariebrainware.com
- Please include detailed steps to reproduce the issue
- Do not publicly disclose vulnerabilities until patched

## Compliance

This application implements security controls aligned with:
- OWASP Top 10 Web Application Security Risks
- OWASP Authentication Cheat Sheet
- OWASP Password Storage Cheat Sheet
- OWASP Session Management Cheat Sheet

## Additional Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [Argon2 Specification](https://github.com/P-H-C/phc-winner-argon2)
- [HSTS Documentation](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Strict-Transport-Security)
- [JWT Best Practices](https://tools.ietf.org/html/rfc8725)
