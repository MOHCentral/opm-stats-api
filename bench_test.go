package main

import (
	"fmt"
	"testing"
)

func BenchmarkSprintfGuid(b *testing.B) {
	guid := "f05bb4e3d92b13ed8c29b2b545d62d02"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf("multikill:%s", guid)
	}
}

func BenchmarkConcatGuid(b *testing.B) {
	guid := "f05bb4e3d92b13ed8c29b2b545d62d02"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = "multikill:" + guid
	}
}
