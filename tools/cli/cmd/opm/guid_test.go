package main

import (
	"testing"
)

func TestIsInvalidGUID(t *testing.T) {
	tests := []struct {
		guid     string
		expected bool
	}{
		{"GUID_ELGAN", true},
		{"SIMPLEGUID", true},
		{"", true},
		{"12345678-1234-1234-1234-1234567890ab", false}, // Valid UUID
		{"some-other-guid", false}, // Has hyphens, technically "valid" by current simple check
	}

	for _, test := range tests {
		if result := IsInvalidGUID(test.guid); result != test.expected {
			t.Errorf("IsInvalidGUID(%q) = %v; expected %v", test.guid, result, test.expected)
		}
	}
}
