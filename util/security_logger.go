package util

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// SecurityEventType represents different types of security events
type SecurityEventType string

const (
	EventLoginSuccess       SecurityEventType = "LOGIN_SUCCESS"
	EventLoginFailure       SecurityEventType = "LOGIN_FAILURE"
	EventSignupSuccess      SecurityEventType = "SIGNUP_SUCCESS"
	EventLogout             SecurityEventType = "LOGOUT"
	EventAccountLocked      SecurityEventType = "ACCOUNT_LOCKED"
	EventPasswordChanged    SecurityEventType = "PASSWORD_CHANGED"
	EventUnauthorizedAccess SecurityEventType = "UNAUTHORIZED_ACCESS"
	EventRateLimitExceeded  SecurityEventType = "RATE_LIMIT_EXCEEDED"
	EventSuspiciousActivity SecurityEventType = "SUSPICIOUS_ACTIVITY"
)

// SecurityEvent represents a security event to be logged
type SecurityEvent struct {
	Timestamp time.Time
	EventType SecurityEventType
	UserID    string
	Email     string
	IP        string
	UserAgent string
	Message   string
	Details   map[string]interface{}
}

var securityLogger *log.Logger

func init() {
	// Initialize security logger - in production, this could write to a separate file
	securityLogger = log.New(os.Stdout, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix)
}

// sanitizeLogValue removes newlines and other characters that could break log parsing
func sanitizeLogValue(value string) string {
	// Replace newlines, carriage returns, and tabs with spaces
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\t", " ")
	// Truncate very long values to prevent log flooding
	if len(value) > 200 {
		value = value[:200] + "..."
	}
	return value
}

// LogSecurityEvent logs a security event
func LogSecurityEvent(event SecurityEvent) {
	event.Timestamp = time.Now()
	
	// Sanitize all string fields to prevent log injection
	msg := fmt.Sprintf("Event=%s UserID=%s Email=%s IP=%s UserAgent=%s Message=%s",
		sanitizeLogValue(string(event.EventType)),
		sanitizeLogValue(event.UserID),
		sanitizeLogValue(event.Email),
		sanitizeLogValue(event.IP),
		sanitizeLogValue(event.UserAgent),
		sanitizeLogValue(event.Message),
	)
	
	if len(event.Details) > 0 {
		// Don't log Details map directly to avoid injection
		// Instead, log the count of details
		msg = fmt.Sprintf("%s DetailsCount=%d", msg, len(event.Details))
	}
	
	securityLogger.Println(msg)
}

// LogLoginSuccess logs a successful login event
func LogLoginSuccess(userID uint, email, ip, userAgent string) {
	LogSecurityEvent(SecurityEvent{
		EventType: EventLoginSuccess,
		UserID:    fmt.Sprintf("%d", userID),
		Email:     email,
		IP:        ip,
		UserAgent: userAgent,
		Message:   "User logged in successfully",
	})
}

// LogLoginFailure logs a failed login attempt
func LogLoginFailure(email, ip, userAgent, reason string) {
	LogSecurityEvent(SecurityEvent{
		EventType: EventLoginFailure,
		Email:     email,
		IP:        ip,
		UserAgent: userAgent,
		Message:   fmt.Sprintf("Login failed: %s", reason),
	})
}

// LogLogout logs a logout event
func LogLogout(userID uint, email, ip, userAgent string) {
	LogSecurityEvent(SecurityEvent{
		EventType: EventLogout,
		UserID:    fmt.Sprintf("%d", userID),
		Email:     email,
		IP:        ip,
		UserAgent: userAgent,
		Message:   "User logged out",
	})
}

// LogAccountLocked logs when an account is locked
func LogAccountLocked(userID uint, email, ip string, reason string) {
	LogSecurityEvent(SecurityEvent{
		EventType: EventAccountLocked,
		UserID:    fmt.Sprintf("%d", userID),
		Email:     email,
		IP:        ip,
		Message:   fmt.Sprintf("Account locked: %s", reason),
	})
}

// LogUnauthorizedAccess logs unauthorized access attempts
func LogUnauthorizedAccess(userID string, email, ip, resource, reason string) {
	LogSecurityEvent(SecurityEvent{
		EventType: EventUnauthorizedAccess,
		UserID:    userID,
		Email:     email,
		IP:        ip,
		Message:   fmt.Sprintf("Unauthorized access to %s: %s", resource, reason),
	})
}

// LogRateLimitExceeded logs when rate limit is exceeded
func LogRateLimitExceeded(email, ip, endpoint string) {
	LogSecurityEvent(SecurityEvent{
		EventType: EventRateLimitExceeded,
		Email:     email,
		IP:        ip,
		Message:   fmt.Sprintf("Rate limit exceeded for endpoint: %s", endpoint),
	})
}
