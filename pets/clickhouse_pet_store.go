package pets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

func NewClickHousePetStore(db clickhouse.Conn) *ClickHousePetStore {
	return &ClickHousePetStore{db: db}
}

type ClickHousePetStore struct {
	db clickhouse.Conn
}

func (s *ClickHousePetStore) RecordPet(ctx context.Context, pet Pet) (string, error) {
	id := uuid.New()
	err := s.db.Exec(ctx, `
		INSERT INTO pets (id, name, species, breed, age, weight_kg, ingested_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, pet.Name, pet.Species, pet.Breed, pet.Age, pet.WeightKG, time.Now(),
	)
	if err != nil {
		return "", fmt.Errorf("insert pet %q: %w", pet.Name, err)
	}
	return id.String(), nil
}

func (s *ClickHousePetStore) GetPet(ctx context.Context, id string) (Pet, error) {
	var p Pet
	err := s.db.QueryRow(ctx,
		`SELECT name, species, breed, age, weight_kg FROM pets WHERE id = ? LIMIT 1`,
		id,
	).Scan(&p.Name, &p.Species, &p.Breed, &p.Age, &p.WeightKG)
	if err != nil {
		return Pet{}, fmt.Errorf("get pet %q: %w", id, err)
	}
	p.ID = id
	return p, nil
}

func (s *ClickHousePetStore) GetAllPets(ctx context.Context, filter PetFilter) ([]Pet, error) {
	var conditions []string
	var args []any

	if filter.Species != "" {
		conditions = append(conditions, "species = ?")
		args = append(args, filter.Species)
	}
	if filter.Breed != "" {
		conditions = append(conditions, "breed = ?")
		args = append(args, filter.Breed)
	}

	query := "SELECT name, species, breed, age, weight_kg FROM pets"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY ingested_at DESC LIMIT ?"
	args = append(args, filter.Limit)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pets: %w", err)
	}
	defer rows.Close()

	var pets []Pet
	for rows.Next() {
		var p Pet
		if err := rows.Scan(&p.Name, &p.Species, &p.Breed, &p.Age, &p.WeightKG); err != nil {
			return nil, fmt.Errorf("scan pet: %w", err)
		}
		pets = append(pets, p)
	}
	return pets, nil
}
