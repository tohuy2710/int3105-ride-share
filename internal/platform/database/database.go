package database

import (
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"int3105-ride-share/internal/platform/config"
	"time"
)

func Open() (*sql.DB, error) {
	n, err := config.PositiveInt("DB_MAX_OPEN_CONNS", 10)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("pgx", config.Env("DATABASE_URL", "postgres://ride:ride_local@localhost:5432/rides?sslmode=disable"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(n)
	db.SetMaxIdleConns(n)
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}
