package worker

import (
	"fmt"
	"testing"
	"strconv"
)

func BenchmarkKeySprintf(b *testing.B) {
	smfID := int64(12345)
	statName := "headshots"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf("stats:smf:%d:%s", smfID, statName)
	}
}

func BenchmarkKeyConcat(b *testing.B) {
	smfID := int64(12345)
	statName := "headshots"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = "stats:smf:" + strconv.FormatInt(smfID, 10) + ":" + statName
	}
}
