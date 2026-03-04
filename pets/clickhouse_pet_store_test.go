package pets_test

import (
	"context"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"petpipeline/pets"
)

func connectClickHouse(t *testing.T) clickhouse.Conn {
	t.Helper()
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"127.0.0.1:9000"},
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
	// addr := os.Getenv("CLICKHOUSE_ADDR")
	// if addr == "" {
	// 	addr = "localhost:9000"
	// }
	// conn, err := clickhouse.Open(&clickhouse.Options{
	// 	Addr: []string{addr},
	// })
	if err != nil {
		t.Skipf("skipping ClickHouse tests: %v", err)
	}
	if err := conn.Ping(context.Background()); err != nil {
		t.Skipf("skipping ClickHouse tests: cannot ping: %v", err)
	}
	return conn
}

func TestClickHousePetStore_RecordPet(t *testing.T) {
	db := connectClickHouse(t)
	store, _ := pets.NewClickHousePetStore(db)

	pet := pets.Pet{Name: "TestBuddy", Species: "Dog", Breed: "Labrador", Age: 3, Weight_KG: 30.0}
	if _, ok := store.RecordPet(pet); !ok {
		t.Error("expected RecordPet to return true")
	}
}

func TestClickHousePetStore_GetAllPets(t *testing.T) {
	db := connectClickHouse(t)
	store, _ := pets.NewClickHousePetStore(db)

	pet := pets.Pet{Name: "FilterTestDog", Species: "Dog", Breed: "Poodle", Age: 2, Weight_KG: 10.0}
	store.RecordPet(pet)

	t.Run("returns pets with no filter", func(t *testing.T) {
		results, count := store.GetAllPets(pets.PetFilter{Limit: 10})
		if count == 0 {
			t.Error("expected at least one pet")
		}
		if len(results) != count {
			t.Errorf("slice length %d != count %d", len(results), count)
		}
	})

	t.Run("filters by species", func(t *testing.T) {
		results, _ := store.GetAllPets(pets.PetFilter{Species: "Dog", Limit: 10})
		for _, p := range results {
			if p.Species != "Dog" {
				t.Errorf("expected species Dog, got %q", p.Species)
			}
		}
	})

	t.Run("filters by breed", func(t *testing.T) {
		results, _ := store.GetAllPets(pets.PetFilter{Breed: "Poodle", Limit: 10})
		for _, p := range results {
			if p.Breed != "Poodle" {
				t.Errorf("expected breed Poodle, got %q", p.Breed)
			}
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		_, count := store.GetAllPets(pets.PetFilter{Limit: 1})
		if count > 1 {
			t.Errorf("expected at most 1 result, got %d", count)
		}
	})
}

func TestClickHousePetStore_GetPet(t *testing.T) {
	db := connectClickHouse(t)
	store, _ := pets.NewClickHousePetStore(db)

	t.Run("returns false for non-existent id", func(t *testing.T) {
		_, found := store.GetPet("00000000-0000-0000-0000-000000000000")
		if found {
			t.Error("expected not found for unknown id")
		}
	})
}
