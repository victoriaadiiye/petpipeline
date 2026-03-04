package infra

import (
	"context"
	"log"
	"os"
	"petpipeline/pets"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func ConnectClickHouse() *pets.ClickHousePetStore {
	chAddr := os.Getenv("CLICKHOUSE_ADDR")
	if chAddr == "" {
		chAddr = "127.0.0.1:9000"
	}
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{chAddr},
		Auth: clickhouse.Auth{
			Database: "app",
			Username: "dev",
			Password: "dev",
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout:     5 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 10 * time.Minute,
	})
	if err != nil {
		log.Fatalf("failed to open clickhouse: %v", err)
	}
	if err := conn.Ping(context.Background()); err != nil {
		log.Fatalf("clickhouse ping: %v", err)
	}

	store, err := pets.NewClickHousePetStore(conn)
	if err != nil {
		log.Fatalf("failed to create clickhouse pet store: %v", err)
	}
	log.Printf("connected to ClickHouse at %s", chAddr)
	return store
}
