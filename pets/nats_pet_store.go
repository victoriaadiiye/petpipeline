package pets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
)

const petSubject = "pets.ingest"

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
	if _, err := s.js.Publish(ctx, petSubject, data); err != nil {
		return "", fmt.Errorf("publish pet to NATS: %w", err)
	}
	return "", nil
}
