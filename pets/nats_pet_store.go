package pets

import (
	"context"
	"encoding/json"
	"log"

	"github.com/nats-io/nats.go/jetstream"
)

const petSubject = "pets.ingest"

func NewNatsPetStore(js jetstream.JetStream) *NatsPetStore {
	return &NatsPetStore{js: js}
}

type NatsPetStore struct {
	js jetstream.JetStream
}

// RecordPet publishes a pet record to the NATS stream.
func (s *NatsPetStore) RecordPet(pet Pet) (string, bool) {
	data, err := json.Marshal(pet)
	if err != nil {
		log.Printf("failed to marshal pet: %v", err)
		return "", false
	}

	if _, err := s.js.Publish(context.Background(), petSubject, data); err != nil {
		log.Printf("failed to publish pet to NATS: %v", err)
		return "", false
	}
	return "", true
}
