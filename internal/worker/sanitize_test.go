package worker

import (
	"testing"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"No colors", "PlayerName", "PlayerName"},
		{"Start color", "^1Player", "Player"},
		{"Middle color", "Player^2Name", "PlayerName"},
		{"End color", "Player^3", "Player"},
		{"Multiple colors", "^1Red^2Green^3Blue", "RedGreenBlue"},
		{"Repeated carats", "^^^1Name", "^^Name"},
		{"Not a color code", "Player^aName", "Player^aName"},
		{"Lonely carat", "Player^", "Player^"},
		{"Digit without carat", "Player1", "Player1"},
		{"Complex", "^1Player^2 ^3Name^0", "Player Name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeName(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func BenchmarkSanitizeName(b *testing.B) {
	input := "^1Player^2Name^3With^4Colors"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeName(input)
	}
}
