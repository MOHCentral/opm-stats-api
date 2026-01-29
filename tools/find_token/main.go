package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

func main() {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, "postgres://mohaa:AvWq6gPAlSfbdpGH@localhost:5432/mohaa_stats?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(ctx)

	var token, id string
	var isActive bool
	err = conn.QueryRow(ctx, "SELECT token, id, is_active FROM servers WHERE name = 'ChartTestServer'").Scan(&token, &id, &isActive)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TOKEN: %s\nID: %s\nACTIVE: %v\n", token, id, isActive)
}
