package main

import (
	"context"
	"fmt"
	"log"

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

	fmt.Println("Running Map Popularity Test Query...")
	rows, err := conn.Query(ctx, `
		SELECT match_id, map_name FROM raw_events LIMIT 10
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var id string
		var name string
		rows.Scan(&id, &name)
		fmt.Printf("Match: %s, Map: %s\n", id, name)
	}
	if !found {
		fmt.Println("NO ROWS FOUND")
	}

	fmt.Println("\nChecking all distinct map_names:")
	rows2, _ := conn.Query(ctx, "SELECT DISTINCT map_name FROM raw_events")
	for rows2.Next() {
		var name string
		rows2.Scan(&name)
		fmt.Printf("Distinct Map: %q\n", name)
	}
}
