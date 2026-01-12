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
