package worker

import (
	"testing"
	"github.com/google/uuid"
)

var defaultNamespace = uuid.MustParse("00000000-0000-0000-0000-000000000000")

func BenchmarkUUIDMustParse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = uuid.MustParse("00000000-0000-0000-0000-000000000000")
	}
}

func BenchmarkUUIDGlobal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = defaultNamespace
	}
}
