package logic

import (
	"strings"
	"testing"
	"time"
)

func TestBuildStatsQuery(t *testing.T) {
	tests := []struct {
		name          string
		req           DynamicQueryRequest
		wantQueryPart string // Check if query contains this
		wantArgsCount int
		wantErr       bool
	}{
		{
			name: "Basic Kills Query",
			req: DynamicQueryRequest{
				Metric: "kills",
				Limit:  10,
			},
			wantQueryPart: "countIf(event_type = 'kill')",
			wantArgsCount: 0,
			wantErr:       false,
		},
		{
			name: "KDR by Weapon",
			req: DynamicQueryRequest{
				Dimension: "weapon",
				Metric:    "kdr",
				Limit:     5,
			},
			wantQueryPart: "GROUP BY extract(extra, 'weapon_([a-zA-Z0-9_]+)')",
			wantArgsCount: 0,
			wantErr:       false,
		},
		{
			name: "Filter by Map and GUID",
			req: DynamicQueryRequest{
				Metric:     "deaths",
				FilterMap:  "dm/mohdm1",
				FilterGUID: "player-123",
			},
			wantQueryPart: "AND actor_id = ? AND map_name = ?",
			wantArgsCount: 2,
			wantErr:       false,
		},
		{
			name: "Invalid Dimension",
			req: DynamicQueryRequest{
				Dimension: "invalid_col",
				Metric:    "kills",
			},
			wantQueryPart: "",
			wantArgsCount: 0,
			wantErr:       true,
		},
		{
			name: "Time Range",
			req: DynamicQueryRequest{
				Metric:    "kills",
				StartDate: time.Now().Add(-1 * time.Hour),
				EndDate:   time.Now(),
			},
			wantQueryPart: "AND timestamp >= ? AND timestamp <= ?",
			wantArgsCount: 2,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuery, gotArgs, err := BuildStatsQuery(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildStatsQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !strings.Contains(gotQuery, tt.wantQueryPart) {
					t.Errorf("BuildStatsQuery() query = %v, want to contain %v", gotQuery, tt.wantQueryPart)
				}
				if len(gotArgs) != tt.wantArgsCount {
					t.Errorf("BuildStatsQuery() args count = %v, want %v", len(gotArgs), tt.wantArgsCount)
				}
			}
		})
	}
}
