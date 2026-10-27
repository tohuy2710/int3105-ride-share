package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"int3105-ride-share/internal/auth/fixtures"
	"int3105-ride-share/internal/contracts"
	"log"
	"os"
	"path/filepath"
)

func main() {
	path := flag.String("out", "fixtures/accounts.json", "fixture output")
	drivers := flag.Int("drivers", 100, "number of unique drivers")
	flag.Parse()
	if *drivers < 2 {
		log.Fatal("at least two drivers required")
	}
	password := os.Getenv("FIXTURE_PASSWORD")
	if password == "" {
		password = "RideShare123!"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	accounts := []fixtures.Account{{User: contracts.User{UserID: "rider_123", Email: "rider@example.com", Role: "RIDER"}, PasswordHash: string(hash)}}
	for i := 1; i <= *drivers; i++ {
		accounts = append(accounts, fixtures.Account{User: contracts.User{UserID: fmt.Sprintf("driver_%d", 455+i), Email: fmt.Sprintf("driver%d@example.com", i), Role: "DRIVER"}, PasswordHash: string(hash)})
	}
	data, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(*path), 0755); err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(*path, append(data, '\n'), 0644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %d fixture accounts to %s", len(accounts), *path)
}
