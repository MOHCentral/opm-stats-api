package worker

import "testing"

func BenchmarkNormalizeWeaponLabel(b *testing.B) {
	weapons := []struct{ w, m, i string }{
		{"Thompson", "MOD_THOMPSON", ""},
		{"", "MOD_GRENADE", ""},
		{"world", "", "explosion"},
		{"Player", "MOD_SUICIDE", ""},
		{"", "", ""},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, w := range weapons {
			_ = normalizeWeaponLabel(w.w, w.m, w.i)
		}
	}
}
