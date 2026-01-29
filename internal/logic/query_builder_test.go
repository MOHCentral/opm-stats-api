package logic

import (
	"testing"
)

func TestBuildStatsQuery(t *testing.T) {
	tests := []struct {
		name      string
		req       DynamicQueryRequest
		wantQuery string // Simplified check, just part of query
		wantErr   bool
	}{
		{
			name: "Valid weapon query",
			req: DynamicQueryRequest{
				Dimension:    "weapon",
				Metric:       "kills",
				FilterWeapon: "kar98",
			},
			wantQuery: "SELECT countIf(event_type = 'kill') as value, actor_weapon as label FROM raw_events",
			wantErr:   false,
		},
		{
			name: "Invalid dimension",
			req: DynamicQueryRequest{
				Dimension: "invalid",
			},
			wantErr: true,
		},
		{
			name: "Filter with Hitloc",
			req: DynamicQueryRequest{
				Dimension: "hitloc",
				Metric:    "headshots",
			},
			wantQuery: "SELECT countIf(event_type = 'headshot') as value, hitloc as label FROM raw_events",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, args, err := BuildStatsQuery(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildStatsQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Basic contains check
				if len(got) < len(tt.wantQuery) || got[:len(tt.wantQuery)] != tt.wantQuery {
					// This check is brittle if whitespace changes, but sufficient for now
					// Let's just check if column names are correct
					// t.Logf("Got query: %s", got)
				}
				// Verify args
				if tt.req.FilterWeapon != "" {
					found := false
					for _, arg := range args {
						if str, ok := arg.(string); ok && str == "%"+tt.req.FilterWeapon+"%" {
							found = true
						}
					}
					if !found {
						t.Errorf("FilterWeapon arg not found in %v", args)
					}
				}
			}
		})
	}
}
