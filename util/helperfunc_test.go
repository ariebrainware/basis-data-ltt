package util

import "testing"

func TestContains(t *testing.T) {
	list := []string{"a", "b", "c"}
	if !Contains("b", list) {
		t.Fatalf("expected Contains to return true for existing item")
	}
	if Contains("x", list) {
		t.Fatalf("expected Contains to return false for missing item")
	}
}

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "trim leading whitespace",
			input:    "  John Doe",
			expected: "John Doe",
		},
		{
			name:     "trim trailing whitespace",
			input:    "John Doe  ",
			expected: "John Doe",
		},
		{
			name:     "trim leading and trailing whitespace",
			input:    "  John Doe  ",
			expected: "John Doe",
		},
		{
			name:     "collapse multiple internal spaces",
			input:    "John  Doe",
			expected: "John Doe",
		},
		{
			name:     "collapse many internal spaces",
			input:    "John     Doe",
			expected: "John Doe",
		},
		{
			name:     "trim and collapse combined",
			input:    "  John    Doe  ",
			expected: "John Doe",
		},
		{
			name:     "already normalized",
			input:    "John Doe",
			expected: "John Doe",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   ",
			expected: "",
		},
		{
			name:     "tabs and newlines",
			input:    "John\t\nDoe",
			expected: "John Doe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
