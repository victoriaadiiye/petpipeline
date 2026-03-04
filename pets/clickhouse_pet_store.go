package pets

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

func NewClickHousePetStore(db clickhouse.Conn) (*ClickHousePetStore, error) {
	return &ClickHousePetStore{db: db}, nil
}

type ClickHousePetStore struct {
	db clickhouse.Conn
}

func (s *ClickHousePetStore) RecordPet(pet Pet) (string, bool) {
	id := uuid.New()
	err := s.db.Exec(context.Background(), `
		INSERT INTO pets (id, name, species, breed, age, weight_kg, ingested_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id,
		pet.Name,
		pet.Species,
		pet.Breed,
		pet.Age,
		pet.Weight_KG,
		time.Now(),
	)
	if err != nil {
		log.Printf("failed to insert pet %q: %v", pet.Name, err)
		return "", false
	}
	return id.String(), true
}

func (s *ClickHousePetStore) GetPet(id string) (Pet, bool) {
	var p Pet
	err := s.db.QueryRow(context.Background(),
		`SELECT name, species, breed, age, weight_kg FROM pets WHERE id = ? LIMIT 1`,
		id,
	).Scan(&p.Name, &p.Species, &p.Breed, &p.Age, &p.Weight_KG)
	if err != nil {
		return Pet{}, false
	}
	p.ID = id
	return p, true
}

func (s *ClickHousePetStore) GetAllPets(filter PetFilter) ([]Pet, int) {
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

	rows, err := s.db.Query(context.Background(), query, args...)

	if err != nil {
		log.Printf("GetAllPets query failed: %v", err)
		return nil, 0
	}
	defer rows.Close()

	var pets []Pet
	for rows.Next() {
		var p Pet
		if err := rows.Scan(&p.Name, &p.Species, &p.Breed, &p.Age, &p.Weight_KG); err != nil {
			log.Printf("GetAllPets scan failed: %v", err)
			return nil, 0
		}
		pets = append(pets, p)
	}
	return pets, len(pets)
}
