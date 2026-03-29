package main

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func BenchmarkSprintf(b *testing.B) {
	smfID := 12345
	statName := "total_kills"
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf("stats:smf:%d:%s", smfID, statName)
	}
}

func BenchmarkConcatStrconv(b *testing.B) {
	smfID := 12345
	statName := "total_kills"
	for i := 0; i < b.N; i++ {
		_ = "stats:smf:" + strconv.Itoa(smfID) + ":" + statName
	}
}

func BenchmarkBuilder(b *testing.B) {
	smfID := 12345
	statName := "total_kills"
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		sb.WriteString("stats:smf:")
		sb.WriteString(strconv.Itoa(smfID))
		sb.WriteString(":")
		sb.WriteString(statName)
		_ = sb.String()
	}
}
