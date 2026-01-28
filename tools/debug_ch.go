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

	var count uint64
	err = conn.QueryRow(ctx, "SELECT count() FROM raw_events").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Total events: %d\n", count)

	rows, err := conn.Query(ctx, "DESCRIBE raw_events")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Columns:")
	for rows.Next() {
		var name, ctype, default_type, default_expr, comment, codec_expr, ttl_expr string
		rows.Scan(&name, &ctype, &default_type, &default_expr, &comment, &codec_expr, &ttl_expr)
		fmt.Printf("- %s: %s\n", name, ctype)
	}
}
