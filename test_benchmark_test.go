package main

import (
	"strings"
	"testing"
)

func sanitizeNameOld(s string) string {
	if !strings.Contains(s, "^") {
		return s
	}

	var sb strings.Builder
	sb.Grow(len(s))

	n := len(s)
	for i := 0; i < n; i++ {
		// Check for color code format ^[0-9]
		if s[i] == '^' && i+1 < n && s[i+1] >= '0' && s[i+1] <= '9' {
			i++ // Skip next char too (the digit)
			continue
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

func sanitizeNameNew(s string) string {
	idx := strings.IndexByte(s, '^')
	if idx == -1 {
		return s
	}

	var sb strings.Builder
	sb.Grow(len(s))

	for {
		// Write everything up to the caret
		sb.WriteString(s[:idx])

		// Advance s to the caret
		s = s[idx:]

		// Check if it's a color code
		if len(s) >= 2 && s[1] >= '0' && s[1] <= '9' {
			// Skip the color code (^ and the digit)
			s = s[2:]
		} else {
			// Not a color code, write the caret
			sb.WriteByte('^')
			s = s[1:]
		}

		if len(s) == 0 {
			break
		}

		idx = strings.IndexByte(s, '^')
		if idx == -1 {
			sb.WriteString(s)
			break
		}
	}
	return sb.String()
}

func BenchmarkSanitizeNameOld(b *testing.B) {
	input := "^1Player^2Name^3With^4Colors"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeNameOld(input)
	}
}

func BenchmarkSanitizeNameNew(b *testing.B) {
	input := "^1Player^2Name^3With^4Colors"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeNameNew(input)
	}
}

func BenchmarkSanitizeNameCleanOld(b *testing.B) {
	input := "PlayerNameWithoutColors"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeNameOld(input)
	}
}

func BenchmarkSanitizeNameCleanNew(b *testing.B) {
	input := "PlayerNameWithoutColors"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeNameNew(input)
	}
}

func BenchmarkSanitizeNameNoColorCodesOld(b *testing.B) {
	input := "Player^Name^With^Carets"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeNameOld(input)
	}
}

func BenchmarkSanitizeNameNoColorCodesNew(b *testing.B) {
	input := "Player^Name^With^Carets"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeNameNew(input)
	}
}
