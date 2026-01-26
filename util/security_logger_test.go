package util

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

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

func TestLogSecurityEvent(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	originalLogger := securityLogger
	securityLogger = log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix)
	defer func() { securityLogger = originalLogger }()

	tests := []struct {
		name     string
		event    SecurityEvent
		contains []string
	}{
		{
			name: "logs basic event",
			event: SecurityEvent{
				EventType: EventLoginSuccess,
				UserID:    "123",
				Email:     "user@example.com",
				IP:        "192.168.1.1",
				UserAgent: "Mozilla/5.0",
				Message:   "Login successful",
			},
			contains: []string{
				"Event=LOGIN_SUCCESS",
				"UserID=123",
				"Email=user@example.com",
				"IP=192.168.1.1",
				"UserAgent=Mozilla/5.0",
				"Message=Login successful",
			},
		},
		{
			name: "sanitizes newlines in message",
			event: SecurityEvent{
				EventType: EventLoginFailure,
				UserID:    "456",
				Email:     "user@example.com",
				IP:        "192.168.1.2",
				UserAgent: "Chrome",
				Message:   "Failed\nlogin\rattempt",
			},
			contains: []string{
				"Event=LOGIN_FAILURE",
				"Message=Failed login attempt",
			},
		},
		{
			name: "logs event with details count",
			event: SecurityEvent{
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
			},
			contains: []string{
				"Event=SUSPICIOUS_ACTIVITY",
				"DetailsCount=2",
			},
		},
		{
			name: "handles empty fields",
			event: SecurityEvent{
				EventType: EventUnauthorizedAccess,
				UserID:    "",
				Email:     "",
				IP:        "10.0.0.2",
				UserAgent: "",
				Message:   "Access denied",
			},
			contains: []string{
				"Event=UNAUTHORIZED_ACCESS",
				"Message=Access denied",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			LogSecurityEvent(tt.event)
			output := buf.String()

			for _, expectedSubstr := range tt.contains {
				if !strings.Contains(output, expectedSubstr) {
					t.Errorf("Log output missing expected substring %q\nGot: %s", expectedSubstr, output)
				}
			}
		})
	}
}

func TestLogLoginSuccess(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := securityLogger
	securityLogger = log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix)
	defer func() { securityLogger = originalLogger }()

	LogLoginSuccess(123, "user@example.com", "192.168.1.1", "Mozilla/5.0")
	output := buf.String()

	expectedStrings := []string{
		"Event=LOGIN_SUCCESS",
		"UserID=123",
		"Email=user@example.com",
		"IP=192.168.1.1",
		"UserAgent=Mozilla/5.0",
		"Message=User logged in successfully",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Log output missing expected substring %q\nGot: %s", expected, output)
		}
	}
}

func TestLogLoginFailure(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := securityLogger
	securityLogger = log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix)
	defer func() { securityLogger = originalLogger }()

	LogLoginFailure("user@example.com", "192.168.1.1", "Mozilla/5.0", "invalid password")
	output := buf.String()

	expectedStrings := []string{
		"Event=LOGIN_FAILURE",
		"Email=user@example.com",
		"IP=192.168.1.1",
		"Message=Login failed: invalid password",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Log output missing expected substring %q\nGot: %s", expected, output)
		}
	}
}

func TestLogLogout(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := securityLogger
	securityLogger = log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix)
	defer func() { securityLogger = originalLogger }()

	LogLogout(456, "user@example.com", "192.168.1.2", "Chrome")
	output := buf.String()

	expectedStrings := []string{
		"Event=LOGOUT",
		"UserID=456",
		"Email=user@example.com",
		"Message=User logged out",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Log output missing expected substring %q\nGot: %s", expected, output)
		}
	}
}

func TestLogAccountLocked(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := securityLogger
	securityLogger = log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix)
	defer func() { securityLogger = originalLogger }()

	LogAccountLocked(789, "locked@example.com", "192.168.1.3", "too many failed attempts")
	output := buf.String()

	expectedStrings := []string{
		"Event=ACCOUNT_LOCKED",
		"UserID=789",
		"Email=locked@example.com",
		"Message=Account locked: too many failed attempts",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Log output missing expected substring %q\nGot: %s", expected, output)
		}
	}
}

func TestLogUnauthorizedAccess(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := securityLogger
	securityLogger = log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix)
	defer func() { securityLogger = originalLogger }()

	LogUnauthorizedAccess("101", "user@example.com", "192.168.1.4", "/admin/users", "insufficient permissions")
	output := buf.String()

	expectedStrings := []string{
		"Event=UNAUTHORIZED_ACCESS",
		"UserID=101",
		"Message=Unauthorized access to /admin/users: insufficient permissions",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Log output missing expected substring %q\nGot: %s", expected, output)
		}
	}
}

func TestLogRateLimitExceeded(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := securityLogger
	securityLogger = log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix)
	defer func() { securityLogger = originalLogger }()

	LogRateLimitExceeded("user@example.com", "192.168.1.5", "/login")
	output := buf.String()

	expectedStrings := []string{
		"Event=RATE_LIMIT_EXCEEDED",
		"Email=user@example.com",
		"IP=192.168.1.5",
		"Message=Rate limit exceeded for endpoint: /login",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Log output missing expected substring %q\nGot: %s", expected, output)
		}
	}
}
