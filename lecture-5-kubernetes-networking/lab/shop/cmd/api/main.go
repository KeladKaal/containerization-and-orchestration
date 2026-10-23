// Command api is the HTTP front of shop: it accepts orders and reads them back.
package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"shop/internal/app"
)

type order struct {
	ID        int64  `json:"id"`
	Item      string `json:"item"`
	Processed bool   `json:"processed"`
}

const schema = `CREATE TABLE IF NOT EXISTS orders (
	id        bigserial PRIMARY KEY,
	item      text NOT NULL,
	processed boolean NOT NULL DEFAULT false
)`

var ordersCreated = app.Counter("shop_orders_created_total", "Orders accepted by api.")

func main() {
	db, err := app.Open()
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	// One table, created on startup: a migration tool would only get in the way
	// of what this lab is really about.
	if _, err := db.Exec(schema); err != nil {
		log.Fatalf("schema: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /order", func(w http.ResponseWriter, r *http.Request) { createOrder(db, w, r) })
	mux.HandleFunc("GET /orders", func(w http.ResponseWriter, r *http.Request) { listOrders(db, w) })
	log.Fatal(app.Serve(mux))
}

func createOrder(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	var in order
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Item == "" {
		http.Error(w, `expected {"item": "..."}`, http.StatusBadRequest)
		return
	}
	if err := db.QueryRow(`INSERT INTO orders (item) VALUES ($1) RETURNING id`, in.Item).Scan(&in.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ordersCreated.Inc()
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, in)
}

func listOrders(db *sql.DB, w http.ResponseWriter) {
	rows, err := db.Query(`SELECT id, item, processed FROM orders ORDER BY id`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	orders := []order{}
	for rows.Next() {
		var o order
		if err := rows.Scan(&o.ID, &o.Item, &o.Processed); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, orders)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}
