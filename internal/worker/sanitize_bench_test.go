package worker

import (
	"strings"
	"testing"
)

func BenchmarkSanitizeNameOld(b *testing.B) {
	input := "^1Player^2Name^3With^4Colors"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeName(input)
	}
}

func sanitizeNameNew(s string) string {
	idx := strings.IndexByte(s, '^')
	if idx == -1 {
		return s
	}

	var sb strings.Builder
	sb.Grow(len(s))

	for {
		sb.WriteString(s[:idx])
		s = s[idx:]

		if len(s) >= 2 && s[1] >= '0' && s[1] <= '9' {
			s = s[2:]
		} else {
			sb.WriteByte('^')
			s = s[1:]
		}

		idx = strings.IndexByte(s, '^')
		if idx == -1 {
			sb.WriteString(s)
			break
		}
	}
	return sb.String()
}

func BenchmarkSanitizeNameNew(b *testing.B) {
	input := "^1Player^2Name^3With^4Colors"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeNameNew(input)
	}
}

func BenchmarkSanitizeNameCleanOld(b *testing.B) {
	input := "CleanPlayerName"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeName(input)
	}
}

func BenchmarkSanitizeNameCleanNew(b *testing.B) {
	input := "CleanPlayerName"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeNameNew(input)
	}
}
