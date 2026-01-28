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

	if err := conn.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping ClickHouse: %v", err)
	}

	generateMapPopularity(ctx, conn)
	generateWeaponUsage(ctx, conn)
}

func generateMapPopularity(ctx context.Context, conn clickhouse.Conn) {
	fmt.Println("Querying map popularity...")
	rows, err := conn.Query(ctx, `
		SELECT map_name, count(DISTINCT match_id) as matches
		FROM raw_events
		WHERE map_name != ''
		GROUP BY map_name
		ORDER BY matches DESC
		LIMIT 10
	`)
	if err != nil {
		log.Printf("Failed to query map popularity: %v", err)
		return
	}
	defer rows.Close()

	var labels []string
	var values []uint64
	var maxVal uint64

	for rows.Next() {
		var label string
		var val uint64
		if err := rows.Scan(&label, &val); err != nil {
			continue
		}
		labels = append(labels, label)
		values = append(values, val)
		if val > maxVal {
			maxVal = val
		}
	}

	if len(labels) == 0 {
		fmt.Println("No data found for map popularity.")
		return
	}

	svg := generateBarChartSVG("Map Popularity (Matches)", labels, values, maxVal, "#4a90e2")
	saveChart("map_popularity.svg", svg)
}

func generateWeaponUsage(ctx context.Context, conn clickhouse.Conn) {
	fmt.Println("Querying weapon usage...")
	rows, err := conn.Query(ctx, `
		SELECT actor_weapon, count() as kills
		FROM raw_events
		WHERE event_type = 'kill' AND actor_weapon != ''
		GROUP BY actor_weapon
		ORDER BY kills DESC
		LIMIT 10
	`)
	if err != nil {
		log.Printf("Failed to query weapon usage: %v", err)
		return
	}
	defer rows.Close()

	var labels []string
	var values []uint64
	var maxVal uint64

	for rows.Next() {
		var label string
		var val uint64
		if err := rows.Scan(&label, &val); err != nil {
			continue
		}
		labels = append(labels, label)
		values = append(values, val)
		if val > maxVal {
			maxVal = val
		}
	}

	if len(labels) == 0 {
		fmt.Println("No data found for weapon usage.")
		return
	}

	svg := generateBarChartSVG("Top Weapons (Kills)", labels, values, maxVal, "#e74c3c")
	saveChart("weapon_usage.svg", svg)
}

func saveChart(filename string, svg string) {
	err := os.MkdirAll("web/static/img", 0755)
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile("web/static/img/"+filename, []byte(svg), 0644)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Chart generated: web/static/img/%s\n", filename)
}

func generateBarChartSVG(title string, labels []string, values []uint64, maxVal uint64, color string) string {
	width := 600
	height := 400
	padding := 50
	barWidth := (width - 2*padding) / len(labels)
	maxBarHeight := height - 2*padding

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg">`, width, height, width, height))
	
	// Background
	sb.WriteString(`<rect width="100%" height="100%" fill="#1a1a1a" />`)
	
	// Title
	sb.WriteString(fmt.Sprintf(`<text x="%d" y="30" fill="white" font-family="Arial" font-size="20" text-anchor="middle">%s</text>`, width/2, title))

	for i, val := range values {
		barHeight := 0
		if maxVal > 0 {
			barHeight = int((val * uint64(maxBarHeight)) / maxVal)
		}
		x := padding + i*barWidth
		y := height - padding - barHeight
		
		// Bar
		sb.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="%s" rx="4" />`, x+5, y, barWidth-10, barHeight, color))
		
		// Label (rotated)
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" fill="white" font-family="Arial" font-size="12" text-anchor="end" transform="rotate(-45 %d %d)">%s</text>`, x+barWidth/2, height-padding+20, x+barWidth/2, height-padding+20, labels[i]))
		
		// Value on top
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" fill="white" font-family="Arial" font-size="10" text-anchor="middle">%d</text>`, x+barWidth/2, y-5, val))
	}

	// X-axis
	sb.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="white" stroke-width="2" />`, padding, height-padding, width-padding, height-padding))

	sb.WriteString(`</svg>`)
	return sb.String()
}
