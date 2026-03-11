package main

import (
	"bytes"
	"testing"
)

var cleanPayload = bytes.Repeat([]byte("test payload clean data without nulls\n"), 100)
var dirtyPayload = append(cleanPayload, 0)

func BenchmarkReplaceAllClean(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = bytes.ReplaceAll(cleanPayload, []byte{0}, []byte{})
	}
}

func BenchmarkReplaceAllCleanGuarded(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var out []byte
		if bytes.IndexByte(cleanPayload, 0) != -1 {
			out = bytes.ReplaceAll(cleanPayload, []byte{0}, []byte{})
		} else {
			out = cleanPayload
		}
		_ = out
	}
}
