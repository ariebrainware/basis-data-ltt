package util

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

// setupTestLogger creates a test logger that captures output and returns it for assertions
// along with a cleanup function to restore the original logger
func setupTestLogger() (*bytes.Buffer, func()) {
	buf := &bytes.Buffer{}
	originalLogger := securityLogger
	securityLogger = log.New(buf, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix)
	cleanup := func() {
		securityLogger = originalLogger
	}
	return buf, cleanup
}

// assertLogContains checks if the log output contains all expected substrings
func assertLogContains(t *testing.T, output string, expected []string) {
	for _, expectedSubstr := range expected {
		if !strings.Contains(output, expectedSubstr) {
			t.Errorf("Log output missing expected substring %q\nGot: %s", expectedSubstr, output)
		}
	}
}

func TestSanitizeLogValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes newlines",
			input:    "hello\nworld",
			expected: "hello world",
		},
		{
			name:     "removes carriage returns",
			input:    "hello\rworld",
			expected: "hello world",
		},
		{
			name:     "removes tabs",
			input:    "hello\tworld",
			expected: "hello world",
		},
		{
			name:     "truncates long values",
			input:    strings.Repeat("a", 250),
			expected: strings.Repeat("a", 200) + "...",
		},
		{
			name:     "handles normal strings",
			input:    "normal string",
			expected: "normal string",
		},
		{
			name:     "handles empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "combines multiple issues",
			input:    "line1\nline2\rline3\ttab",
			expected: "line1 line2 line3 tab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeLogValue(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeLogValue() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestLogSecurityEventBasic(t *testing.T) {
	buf, cleanup := setupTestLogger()
	defer cleanup()

	LogSecurityEvent(SecurityEvent{
		EventType: EventLoginSuccess,
		UserID:    "123",
		Email:     "user@example.com",
		IP:        "192.168.1.1",
		UserAgent: "Mozilla/5.0",
		Message:   "Login successful",
	})

	assertLogContains(t, buf.String(), []string{
		"Event=LOGIN_SUCCESS",
		"UserID=123",
		"Email=user@example.com",
		"IP=192.168.1.1",
		"UserAgent=Mozilla/5.0",
		"Message=Login successful",
	})
}

func TestLogSecurityEventSanitization(t *testing.T) {
	buf, cleanup := setupTestLogger()
	defer cleanup()

	LogSecurityEvent(SecurityEvent{
		EventType: EventLoginFailure,
		UserID:    "456",
		Email:     "user@example.com",
		IP:        "192.168.1.2",
		UserAgent: "Chrome",
		Message:   "Failed\nlogin\rattempt",
	})

	assertLogContains(t, buf.String(), []string{
		"Event=LOGIN_FAILURE",
		"Message=Failed login attempt",
	})
}

func TestLogSecurityEventWithDetails(t *testing.T) {
	buf, cleanup := setupTestLogger()
	defer cleanup()

	LogSecurityEvent(SecurityEvent{
		EventType: EventSuspiciousActivity,
		UserID:    "789",
		Email:     "suspicious@example.com",
		IP:        "10.0.0.1",
		UserAgent: "Bot",
		Message:   "Suspicious activity detected",
		Details: map[string]interface{}{
			"reason": "multiple IPs",
			"count":  5,
		},
	})

	assertLogContains(t, buf.String(), []string{
		"Event=SUSPICIOUS_ACTIVITY",
		"DetailsCount=2",
	})
}

func TestLogSecurityEventEmptyFields(t *testing.T) {
	buf, cleanup := setupTestLogger()
	defer cleanup()

	LogSecurityEvent(SecurityEvent{
		EventType: EventUnauthorizedAccess,
		UserID:    "",
		Email:     "",
		IP:        "10.0.0.2",
		UserAgent: "",
		Message:   "Access denied",
	})

	assertLogContains(t, buf.String(), []string{
		"Event=UNAUTHORIZED_ACCESS",
		"Message=Access denied",
	})
}

func TestLoginLogging(t *testing.T) {
	tests := []struct {
		name     string
		logFunc  func()
		contains []string
	}{
		{
			name: "LogLoginSuccess",
			logFunc: func() {
				LogLoginSuccess(LoginParams{UserID: 123, Email: "user@example.com", IP: "192.168.1.1", UserAgent: "Mozilla/5.0"})
			},
			contains: []string{
				"Event=LOGIN_SUCCESS",
				"UserID=123",
				"Email=user@example.com",
				"IP=192.168.1.1",
				"UserAgent=Mozilla/5.0",
				"Message=User logged in successfully",
			},
		},
		{
			name: "LogLoginFailure",
			logFunc: func() {
				LogLoginFailure(LoginParams{Email: "user@example.com", IP: "192.168.1.1", UserAgent: "Mozilla/5.0", Reason: "invalid password"})
			},
			contains: []string{
				"Event=LOGIN_FAILURE",
				"Email=user@example.com",
				"IP=192.168.1.1",
				"Message=Login failed: invalid password",
			},
		},
		{
			name: "LogLogout",
			logFunc: func() {
				LogLogout(LoginParams{UserID: 456, Email: "user@example.com", IP: "192.168.1.2", UserAgent: "Chrome"})
			},
			contains: []string{
				"Event=LOGOUT",
				"UserID=456",
				"Email=user@example.com",
				"Message=User logged out",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, cleanup := setupTestLogger()
			defer cleanup()

			tt.logFunc()
			assertLogContains(t, buf.String(), tt.contains)
		})
	}
}

func TestAccountAndAccessLogging(t *testing.T) {
	tests := []struct {
		name     string
		logFunc  func()
		contains []string
	}{
		{
			name: "LogAccountLocked",
			logFunc: func() {
				LogAccountLocked(AccountLockParams{UserID: 789, Email: "locked@example.com", IP: "192.168.1.3", Reason: "too many failed attempts"})
			},
			contains: []string{
				"Event=ACCOUNT_LOCKED",
				"UserID=789",
				"Email=locked@example.com",
				"Message=Account locked: too many failed attempts",
			},
		},
		{
			name: "LogUnauthorizedAccess",
			logFunc: func() {
				LogUnauthorizedAccess(UnauthorizedAccessParams{
					UserID:   "101",
					Email:    "user@example.com",
					IP:       "192.168.1.4",
					Resource: "/admin/users",
					Reason:   "insufficient permissions",
				})
			},
			contains: []string{
				"Event=UNAUTHORIZED_ACCESS",
				"UserID=101",
				"Message=Unauthorized access to /admin/users: insufficient permissions",
			},
		},
		{
			name: "LogRateLimitExceeded",
			logFunc: func() {
				LogRateLimitExceeded(RateLimitParams{Email: "user@example.com", IP: "192.168.1.5", Endpoint: "/login"})
			},
			contains: []string{
				"Event=RATE_LIMIT_EXCEEDED",
				"Email=user@example.com",
				"IP=192.168.1.5",
				"Message=Rate limit exceeded for endpoint: /login",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, cleanup := setupTestLogger()
			defer cleanup()

			tt.logFunc()
			assertLogContains(t, buf.String(), tt.contains)
		})
	}
}
