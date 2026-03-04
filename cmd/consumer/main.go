package main

import (
	"context"
	"encoding/json"
	"log"
	"petpipeline/internal/infra"
	"petpipeline/pets"

	"github.com/nats-io/nats.go/jetstream"
)

func processMsg(msg jetstream.Msg, store pets.PetWriter) bool {
	var pet pets.Pet
	if err := json.Unmarshal(msg.Data(), &pet); err != nil {
		return false // caller will Nak or Term
	}
	_, ok := store.RecordPet(pet)
	return ok
}

func main() {
	_, js, natsCleanup := infra.ConnectNATS()
	defer natsCleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Bind to the existing PETS stream as a consumer
	cons, err := js.CreateOrUpdateConsumer(ctx, "PETS", jetstream.ConsumerConfig{
		Name:    "pet-consumer",
		Durable: "pet-consumer", // durable = survives restarts
	})
	if err != nil {
		log.Fatalf("failed to create consumer: %v", err)
	}

	store := infra.ConnectClickHouse()

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		success := processMsg(msg, store)
		if success {
			log.Println("Message successfully processed and acked.")
			msg.Ack()
			return
		}
		meta, err := msg.Metadata()
		if err != nil || meta.NumDelivered >= 5 {
			log.Printf("Message failed after %d attempts, terminating.", meta.NumDelivered)
			msg.Term()
			return
		}
		log.Printf("Message failed (attempt %d/5), nacking for retry.", meta.NumDelivered)
		msg.Nak()
	})
	if err != nil {
		log.Fatalf("failed to start consumer: %v", err)
	}
	defer cc.Stop()

	select {}
}
