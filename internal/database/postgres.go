package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	"warehouse-service/internal/config"

	_ "github.com/lib/pq"
)

func Open(ctx context.Context, cfg config.Database) (*sql.DB, error) {
	query := url.Values{
		"sslmode":         {cfg.SSLMode},
		"connect_timeout": {"5"},
	}
	dsn := (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:     cfg.Name,
		RawQuery: query.Encode(),
	}).String()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL connection: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	return db, nil
}
