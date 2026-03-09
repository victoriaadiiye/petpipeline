package pets

import "context"

type Pet struct {
	ID       string  `json:"id,omitempty"`
	Name     string  `json:"name"`
	Species  string  `json:"species"`
	Breed    string  `json:"breed"`
	Age      uint8   `json:"age"`
	WeightKG float32 `json:"weight_kg"`
}

type PetWriter interface {
	RecordPet(ctx context.Context, pet Pet) (string, error)
}

type PetFilter struct {
	Species string
	Breed   string
	Limit   int
}

type PetReader interface {
	GetPet(ctx context.Context, id string) (Pet, error)
	GetAllPets(ctx context.Context, filter PetFilter) ([]Pet, error)
}
