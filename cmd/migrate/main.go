package main

import (
	"context"
	"int3105-ride-share/internal/platform/database"
	"int3105-ride-share/migrations"
	"log"
	"time"
)

func main() {
	db, err := database.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = migrations.Apply(ctx, db); err != nil {
		log.Fatal(err)
	}
	log.Print("migrations applied")
}
