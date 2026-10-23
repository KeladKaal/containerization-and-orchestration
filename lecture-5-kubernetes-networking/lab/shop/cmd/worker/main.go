// Command worker picks up unprocessed orders and marks them processed.
package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"shop/internal/app"
)

// FOR UPDATE SKIP LOCKED is what lets several worker replicas share one table:
// each transaction claims rows nobody else is holding instead of waiting on them.
const claim = `UPDATE orders SET processed = true WHERE id IN (
	SELECT id FROM orders WHERE NOT processed ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED
)`

var ordersProcessed = app.Counter("shop_orders_processed_total", "Orders marked processed by worker.")

func main() {
	db, err := app.Open()
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	go process(db)
	// The worker serves no traffic, but it still answers /health and /metrics:
	// the probes and the ServiceMonitor need somewhere to knock.
	log.Fatal(app.Serve(http.NewServeMux()))
}

func process(db *sql.DB) {
	for range time.Tick(2 * time.Second) {
		res, err := db.Exec(claim, 10)
		if err != nil {
			log.Printf("claim orders: %v", err)
			continue
		}
		if n, _ := res.RowsAffected(); n > 0 {
			ordersProcessed.Add(float64(n))
			log.Printf("processed %d orders", n)
		}
	}
}
