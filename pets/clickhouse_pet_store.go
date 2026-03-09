package pets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

// NewClickHousePetStore creates a store backed by the given ClickHouse table.
// table must be a trusted, internally-configured value (not user input).
func NewClickHousePetStore(db clickhouse.Conn, table string) *ClickHousePetStore {
	return &ClickHousePetStore{db: db, table: table}
}

type ClickHousePetStore struct {
	db    clickhouse.Conn
	table string
}

func (s *ClickHousePetStore) RecordPet(ctx context.Context, pet Pet) (string, error) {
	id := uuid.New()
	err := s.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (id, name, species, breed, age, weight_kg, ingested_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, s.table),
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
		fmt.Sprintf(`SELECT name, species, breed, age, weight_kg FROM %s WHERE id = ? LIMIT 1`, s.table),
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

	query := fmt.Sprintf("SELECT name, species, breed, age, weight_kg FROM %s", s.table)
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

// SpeciesStore pairs a species name with its ClickHouse store.
type SpeciesStore struct {
	Species string
	Store   *ClickHousePetStore
}

// MultiPetReader fans out reads across multiple species-specific stores.
type MultiPetReader struct {
	stores []SpeciesStore
}

func NewMultiPetReader(stores ...SpeciesStore) *MultiPetReader {
	return &MultiPetReader{stores: stores}
}

// GetPet searches all stores for the given id, returning the first match.
func (m *MultiPetReader) GetPet(ctx context.Context, id string) (Pet, error) {
	for _, ss := range m.stores {
		p, err := ss.Store.GetPet(ctx, id)
		if err == nil {
			return p, nil
		}
	}
	return Pet{}, fmt.Errorf("pet %q not found", id)
}

// GetAllPets merges results across stores. When filter.Species is set only
// the matching store is queried.
func (m *MultiPetReader) GetAllPets(ctx context.Context, filter PetFilter) ([]Pet, error) {
	var all []Pet
	for _, ss := range m.stores {
		if filter.Species != "" && !strings.EqualFold(filter.Species, ss.Species) {
			continue
		}
		pets, err := ss.Store.GetAllPets(ctx, filter)
		if err != nil {
			return nil, err
		}
		all = append(all, pets...)
	}
	return all, nil
}
