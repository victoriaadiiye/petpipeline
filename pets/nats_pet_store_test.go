package pets_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"petpipeline/pets"
)

func runTestNATSServer(t *testing.T) *natsserver.Server {
	t.Helper()
	opts := &natsserver.Options{
		JetStream: true,
		Port:      -1,
	}
	srv, err := natsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("failed to create NATS server: %v", err)
	}
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready in time")
	}
	t.Cleanup(srv.Shutdown)
	return srv
}

func setupNatsStore(t *testing.T) (*pets.NatsPetStore, jetstream.JetStream) {
	t.Helper()
	srv := runTestNATSServer(t)

	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("failed to connect to NATS: %v", err)
	}
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("failed to create JetStream context: %v", err)
	}

	// Create a stream per species matching the subjects used by NatsPetStore.
	for _, cfg := range []jetstream.StreamConfig{
		{Name: "PETS_DOGS", Subjects: []string{"pets.dogs"}},
		{Name: "PETS_CATS", Subjects: []string{"pets.cats"}},
		{Name: "PETS_UNKNOWN", Subjects: []string{"pets.unknown"}},
	} {
		if _, err := js.CreateStream(context.Background(), cfg); err != nil {
			t.Fatalf("failed to create stream %s: %v", cfg.Name, err)
		}
	}

	return pets.NewNatsPetStore(js), js
}

func TestNatsPetStore_RecordPet(t *testing.T) {
	store, js := setupNatsStore(t)

	t.Run("publishes dog to pets.dogs stream", func(t *testing.T) {
		pet := pets.Pet{Name: "Buddy", Species: "Dog", Breed: "Labrador", Age: 3, WeightKG: 30.0}
		if _, err := store.RecordPet(context.Background(), pet); err != nil {
			t.Fatalf("RecordPet failed: %v", err)
		}

		consumer, err := js.CreateOrUpdateConsumer(context.Background(), "PETS_DOGS", jetstream.ConsumerConfig{
			FilterSubject: "pets.dogs",
			DeliverPolicy: jetstream.DeliverAllPolicy,
		})
		if err != nil {
			t.Fatalf("failed to create consumer: %v", err)
		}

		msg, err := consumer.Next(jetstream.FetchMaxWait(2 * time.Second))
		if err != nil {
			t.Fatalf("expected a message on stream, got error: %v", err)
		}

		var got pets.Pet
		if err := json.Unmarshal(msg.Data(), &got); err != nil {
			t.Fatalf("failed to unmarshal message: %v", err)
		}

		if got != pet {
			t.Errorf("published pet mismatch: got %+v, want %+v", got, pet)
		}
	})

	t.Run("publishes cat to pets.cats stream", func(t *testing.T) {
		pet := pets.Pet{Name: "Whiskers", Species: "Cat", Breed: "Siamese", Age: 2, WeightKG: 4.5}
		if _, err := store.RecordPet(context.Background(), pet); err != nil {
			t.Fatalf("RecordPet failed: %v", err)
		}

		consumer, err := js.CreateOrUpdateConsumer(context.Background(), "PETS_CATS", jetstream.ConsumerConfig{
			FilterSubject: "pets.cats",
			DeliverPolicy: jetstream.DeliverAllPolicy,
		})
		if err != nil {
			t.Fatalf("failed to create consumer: %v", err)
		}

		msg, err := consumer.Next(jetstream.FetchMaxWait(2 * time.Second))
		if err != nil {
			t.Fatalf("expected a message on stream, got error: %v", err)
		}

		var got pets.Pet
		if err := json.Unmarshal(msg.Data(), &got); err != nil {
			t.Fatalf("failed to unmarshal message: %v", err)
		}

		if got != pet {
			t.Errorf("published pet mismatch: got %+v, want %+v", got, pet)
		}
	})

	t.Run("returns error when JetStream publish fails", func(t *testing.T) {
		// Use a closed connection to force publish failure.
		srv := runTestNATSServer(t)
		nc2, _ := nats.Connect(srv.ClientURL())
		js2, _ := jetstream.New(nc2)
		js2.CreateStream(context.Background(), jetstream.StreamConfig{
			Name: "PETS_DOGS2", Subjects: []string{"pets.dogs"},
		})
		store2 := pets.NewNatsPetStore(js2)
		nc2.Close() // close before publishing

		pet := pets.Pet{Name: "Ghost", Species: "Dog"}
		if _, err := store2.RecordPet(context.Background(), pet); err == nil {
			t.Error("expected error on closed connection")
		}
	})
}
