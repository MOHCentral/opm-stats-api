package main

import (
	"fmt"
	"strconv"

	"testing"
)

func BenchmarkSprintfAllocs(b *testing.B) {
	smfID := 12345
	statName := "total_kills"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf("stats:smf:%d:%s", smfID, statName)
	}
}

func BenchmarkConcatStrconvAllocs(b *testing.B) {
	smfID := 12345
	statName := "total_kills"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = "stats:smf:" + strconv.Itoa(smfID) + ":" + statName
	}
}
