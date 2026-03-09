package platform

import (
	"context"
	"fmt"
	"log"
	"os"
	"petpipeline/pets"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// ConnectClickHouse opens a ClickHouse connection and wraps it in a store
// backed by the given table name.
func ConnectClickHouse(table string) (*pets.ClickHousePetStore, error) {
	conn, err := openClickHouseConn()
	if err != nil {
		return nil, err
	}
	return pets.NewClickHousePetStore(conn, table), nil
}

// ConnectClickHouseMulti opens a single ClickHouse connection and returns a
// MultiPetReader that fans out reads across the dogs and cats tables.
func ConnectClickHouseMulti() (*pets.MultiPetReader, error) {
	conn, err := openClickHouseConn()
	if err != nil {
		return nil, err
	}
	return pets.NewMultiPetReader(
		pets.SpeciesStore{Species: "Dog", Store: pets.NewClickHousePetStore(conn, "dogs")},
		pets.SpeciesStore{Species: "Cat", Store: pets.NewClickHousePetStore(conn, "cats")},
	), nil
}

func openClickHouseConn() (clickhouse.Conn, error) {
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
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}
	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}
	log.Printf("connected to ClickHouse at %s", chAddr)
	return conn, nil
}
