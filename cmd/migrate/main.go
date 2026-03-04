package main

import (
	"fmt"
	"log"
	"os"
	"petpipeline/db"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/clickhouse"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func main() {
	chAddr := os.Getenv("CLICKHOUSE_ADDR")
	if chAddr == "" {
		chAddr = "127.0.0.1:9000"
	}

	source, err := iofs.New(db.Migrations, "migrations")
	if err != nil {
		log.Fatalf("failed to create migration source: %v", err)
	}
	dsn := fmt.Sprintf("clickhouse://%s?database=app&username=dev&password=dev&x-multi-statement=true", chAddr)
	m, err := migrate.NewWithSourceInstance("iofs", source, dsn)
	if err != nil {
		log.Fatalf("failed to create migrate instance: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migration failed: %v", err)
	}
	log.Println("migrations applied successfully")
}
