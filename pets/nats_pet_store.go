package pets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

// subjectForSpecies maps a species name to its NATS subject.
func subjectForSpecies(species string) string {
	switch strings.ToLower(species) {
	case "dog":
		return "pets.dogs"
	case "cat":
		return "pets.cats"
	default:
		return "pets.unknown"
	}
}

func NewNatsPetStore(js jetstream.JetStream) *NatsPetStore {
	return &NatsPetStore{js: js}
}

type NatsPetStore struct {
	js jetstream.JetStream
}

func (s *NatsPetStore) RecordPet(ctx context.Context, pet Pet) (string, error) {
	data, err := json.Marshal(pet)
	if err != nil {
		return "", fmt.Errorf("marshal pet: %w", err)
	}
	subject := subjectForSpecies(pet.Species)
	if _, err := s.js.Publish(ctx, subject, data); err != nil {
		return "", fmt.Errorf("publish pet to NATS: %w", err)
	}
	return "", nil
}
