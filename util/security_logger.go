package util

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ariebrainware/basis-data-ltt/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
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
	EventEndpointCall       SecurityEventType = "ENDPOINT_CALL"
)

// SecurityEvent represents a security event to be logged
type SecurityEvent struct {
	EventType SecurityEventType
	UserID    string
	Email     string
	IP        string
	UserAgent string
	Message   string
	Details   map[string]interface{}
}

var securityLogger *log.Logger
var securityDB *gorm.DB

// SetSecurityLoggerDB sets a gorm DB instance used by the security logger.
// Call this during application startup (e.g. in main) after DB initialization.
func SetSecurityLoggerDB(db *gorm.DB) {
	securityDB = db
}

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

// formatLocationString creates a standardized location string from city and country
func formatLocationString(city, country string) string {
	if city != "" && country != "" {
		return fmt.Sprintf("%s/%s", city, country)
	} else if country != "" {
		return country
	} else if city != "" {
		return city
	}
	return ""
}

// persistSecurityLog writes a security event to the database (best-effort)
func persistSecurityLog(event SecurityEvent, location string) {
	if securityDB == nil {
		return
	}

	var details datatypes.JSON
	if event.Details != nil {
		if b, err := json.Marshal(event.Details); err == nil {
			details = datatypes.JSON(b)
		}
	}

	entry := model.SecurityLog{
		EventType: string(event.EventType),
		UserID:    event.UserID,
		Email:     sanitizeLogValue(event.Email),
		IP:        sanitizeLogValue(event.IP),
		Location:  sanitizeLogValue(location),
		UserAgent: sanitizeLogValue(event.UserAgent),
		Message:   sanitizeLogValue(event.Message),
		Details:   details,
	}

	// best-effort write; ignore errors but log them to stderr
	if err := securityDB.Create(&entry).Error; err != nil {
		securityLogger.Printf("Failed to persist security event: %v", err)
	}
}

// LogSecurityEvent logs a security event
func LogSecurityEvent(event SecurityEvent) {
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

	// Persist to DB if available (best-effort, do not fail operation)
	loc := GetIPLocation(event.IP)
	location := formatLocationString(loc.City, loc.Country)
	persistSecurityLog(event, location)
}

// (IP lookup implemented in util/geoip.go)

// UnauthorizedAccessParams groups parameters for unauthorized access logging
type UnauthorizedAccessParams struct {
	UserID   string
	Email    string
	IP       string
	Resource string
	Reason   string
}

// LogUnauthorizedAccess logs unauthorized access attempts
func LogUnauthorizedAccess(params UnauthorizedAccessParams) {
	LogSecurityEvent(SecurityEvent{
		EventType: EventUnauthorizedAccess,
		UserID:    params.UserID,
		Email:     params.Email,
		IP:        params.IP,
		Message:   fmt.Sprintf("Unauthorized access to %s: %s", params.Resource, params.Reason),
	})
}

// LoginParams groups parameters for login-related logging
type LoginParams struct {
	UserID    uint
	Email     string
	IP        string
	UserAgent string
	Reason    string // For failures only
}

// logLoginEventWithUserID is a helper to reduce duplication in login-related logging
func logLoginEventWithUserID(eventType SecurityEventType, message string, params LoginParams) {
	LogSecurityEvent(SecurityEvent{
		EventType: eventType,
		UserID:    fmt.Sprintf("%d", params.UserID),
		Email:     params.Email,
		IP:        params.IP,
		UserAgent: params.UserAgent,
		Message:   message,
	})
}

// LogLoginSuccess logs a successful login event
func LogLoginSuccess(params LoginParams) {
	logLoginEventWithUserID(EventLoginSuccess, "User logged in successfully", params)
}

// LogLoginFailure logs a failed login attempt
func LogLoginFailure(params LoginParams) {
	msg := "Login failed"
	if params.Reason != "" {
		msg = fmt.Sprintf("Login failed: %s", params.Reason)
	}
	LogSecurityEvent(SecurityEvent{
		EventType: EventLoginFailure,
		Email:     params.Email,
		IP:        params.IP,
		UserAgent: params.UserAgent,
		Message:   msg,
	})
}

// LogLogout logs a logout event
func LogLogout(params LoginParams) {
	logLoginEventWithUserID(EventLogout, "User logged out", params)
}

// AccountLockParams groups parameters for account lock logging
type AccountLockParams struct {
	UserID uint
	Email  string
	IP     string
	Reason string
}

// LogAccountLocked logs when an account is locked
func LogAccountLocked(params AccountLockParams) {
	LogSecurityEvent(SecurityEvent{
		EventType: EventAccountLocked,
		UserID:    fmt.Sprintf("%d", params.UserID),
		Email:     params.Email,
		IP:        params.IP,
		Message:   fmt.Sprintf("Account locked: %s", params.Reason),
	})
}

// RateLimitParams groups parameters for rate limit logging
type RateLimitParams struct {
	Email    string
	IP       string
	Endpoint string
}

// LogRateLimitExceeded logs when rate limit is exceeded
func LogRateLimitExceeded(params RateLimitParams) {
	LogSecurityEvent(SecurityEvent{
		EventType: EventRateLimitExceeded,
		Email:     params.Email,
		IP:        params.IP,
		Message:   fmt.Sprintf("Rate limit exceeded for endpoint: %s", params.Endpoint),
	})
}

// GetSecurityLoggerForTest returns the current security logger for testing purposes
func GetSecurityLoggerForTest() *log.Logger {
	return securityLogger
}

// SetSecurityLoggerForTest sets a custom logger for testing purposes
func SetSecurityLoggerForTest(logger *log.Logger) {
	securityLogger = logger
}
