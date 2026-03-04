package pets

// Pet represents a pet with its attributes.
type Pet struct {
	ID        string  `json:"id,omitempty"`
	Name      string  `json:"name"`
	Species   string  `json:"species"`
	Breed     string  `json:"breed"`
	Age       uint8   `json:"age"`
	Weight_KG float32 `json:"weight_kg"`
}

type PetWriter interface {
	RecordPet(pet Pet) (string, bool)
}

type PetFilter struct {
	Species string
	Breed   string
	Limit   int
}

type PetReader interface {
	GetPet(id string) (Pet, bool)
	GetAllPets(filter PetFilter) ([]Pet, int)
}
