package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func main() {
	ctx := context.Background()
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"localhost:9000"},
		Auth: clickhouse.Auth{
			Database: "mohaa_stats",
			Username: "default",
			Password: "AvWq6gPAlSfbdpGH",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	migration, err := os.ReadFile("migrations/clickhouse/009_expand_player_stats_mv.sql")
	if err != nil {
		log.Fatal(err)
	}

	statements := strings.Split(string(migration), ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		err = conn.Exec(ctx, stmt)
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("Migration applied successfully!")

	var count uint64
	err = conn.QueryRow(ctx, "SELECT count() FROM mohaa_stats.raw_events").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Total events: %d\n", count)
}
