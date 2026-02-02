package main

import (
"context"
"fmt"
"log"
"os"

"github.com/ClickHouse/clickhouse-go/v2"
)

func main() {
	chURL := os.Getenv("CLICKHOUSE_URL")
	if chURL == "" {
		chURL = "clickhouse://localhost:9000/mohaa_stats"
	}
	
	opts, err := clickhouse.ParseDSN(chURL)
	if err != nil {
		log.Fatalf("Failed to parse DSN: %v", err)
	}
	
	conn, err := clickhouse.Open(opts)
	if err != nil {
		log.Fatalf("Failed to open connection: %v", err)
	}
	defer conn.Close()
	
	ctx := context.Background()
	guid := "player1-guid-abcd1234"
	
	query := `
		SELECT 
			countIf(event_type = 'player_kill' AND actor_id = ?) as kills,
			countIf(event_type = 'player_kill' AND target_id = ?) as deaths
		FROM mohaa_stats.raw_events
		WHERE (actor_id = ? OR target_id = ?)
	`
	
	var kills, deaths uint64
	if err := conn.QueryRow(ctx, query, guid, guid, guid, guid).Scan(&kills, &deaths); err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	
	fmt.Printf("Query result: kills=%d, deaths=%d\n", kills, deaths)
}
