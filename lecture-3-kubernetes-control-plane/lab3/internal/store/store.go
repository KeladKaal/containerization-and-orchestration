package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Order is the representation shared by the API and the worker.
type Order struct {
	ID          int64           `json:"id"`
	Payload     json.RawMessage `json:"payload"`
	Processed   bool            `json:"processed"`
	CreatedAt   time.Time       `json:"created_at"`
	ProcessedAt *time.Time      `json:"processed_at,omitempty"`
}

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	s := &Store{pool: pool}
	if err := s.ensureSchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) ensureSchema(ctx context.Context) error {
	const schema = `
		CREATE TABLE IF NOT EXISTS orders (
			id BIGSERIAL PRIMARY KEY,
			payload JSONB NOT NULL,
			processed BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			processed_at TIMESTAMPTZ
		);
		CREATE INDEX IF NOT EXISTS orders_unprocessed_idx
			ON orders (id) WHERE processed = FALSE;
	`
	if _, err := s.pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("initialize database schema: %w", err)
	}
	return nil
}

func (s *Store) CreateOrder(ctx context.Context, payload json.RawMessage) (Order, error) {
	const query = `
		INSERT INTO orders (payload)
		VALUES ($1)
		RETURNING id, payload, processed, created_at, processed_at
	`
	var order Order
	err := s.pool.QueryRow(ctx, query, payload).Scan(
		&order.ID,
		&order.Payload,
		&order.Processed,
		&order.CreatedAt,
		&order.ProcessedAt,
	)
	if err != nil {
		return Order{}, fmt.Errorf("insert order: %w", err)
	}
	return order, nil
}

func (s *Store) ListOrders(ctx context.Context) ([]Order, error) {
	const query = `
		SELECT id, payload, processed, created_at, processed_at
		FROM orders
		ORDER BY id
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query orders: %w", err)
	}
	defer rows.Close()

	orders := make([]Order, 0)
	for rows.Next() {
		var order Order
		if err := rows.Scan(
			&order.ID,
			&order.Payload,
			&order.Processed,
			&order.CreatedAt,
			&order.ProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}
	return orders, nil
}

// ProcessNext atomically claims and marks one pending order. SKIP LOCKED makes
// this safe when several worker replicas run concurrently.
func (s *Store) ProcessNext(ctx context.Context) (Order, bool, error) {
	const query = `
		WITH next_order AS (
			SELECT id
			FROM orders
			WHERE processed = FALSE
			ORDER BY id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE orders AS o
		SET processed = TRUE, processed_at = NOW()
		FROM next_order
		WHERE o.id = next_order.id
		RETURNING o.id, o.payload, o.processed, o.created_at, o.processed_at
	`

	var order Order
	err := s.pool.QueryRow(ctx, query).Scan(
		&order.ID,
		&order.Payload,
		&order.Processed,
		&order.CreatedAt,
		&order.ProcessedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, false, nil
	}
	if err != nil {
		return Order{}, false, fmt.Errorf("process next order: %w", err)
	}
	return order, true, nil
}
