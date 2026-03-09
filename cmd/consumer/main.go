package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"petpipeline/internal/platform"
	"petpipeline/pets"

	"github.com/nats-io/nats.go/jetstream"
)

func processMsg(ctx context.Context, msg jetstream.Msg, store pets.PetWriter) error {
	var pet pets.Pet
	if err := json.Unmarshal(msg.Data(), &pet); err != nil {
		return fmt.Errorf("unmarshal pet: %w", err)
	}
	if _, err := store.RecordPet(ctx, pet); err != nil {
		return fmt.Errorf("record pet: %w", err)
	}
	return nil
}

func main() {
	stream := os.Getenv("CONSUMER_STREAM")
	if stream == "" {
		stream = "PETS_DOGS"
	}
	table := os.Getenv("CONSUMER_TABLE")
	if table == "" {
		table = "dogs"
	}
	consumerName := os.Getenv("CONSUMER_NAME")
	if consumerName == "" {
		consumerName = strings.ToLower(stream) + "-consumer"
	}

	log.Printf("starting consumer: stream=%s table=%s consumer=%s", stream, table, consumerName)

	_, js, natsCleanup, err := platform.ConnectNATS()
	if err != nil {
		log.Fatalf("NATS: %v", err)
	}
	defer natsCleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cons, err := js.CreateOrUpdateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Name:    consumerName,
		Durable: consumerName,
	})
	if err != nil {
		log.Fatalf("failed to create consumer: %v", err)
	}

	store, err := platform.ConnectClickHouse(table)
	if err != nil {
		log.Fatalf("ClickHouse: %v", err)
	}

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		if err := processMsg(ctx, msg, store); err != nil {
			meta, metaErr := msg.Metadata()
			if metaErr != nil || meta.NumDelivered >= 5 {
				log.Printf("Message failed after max attempts, terminating: %v", err)
				msg.Term()
				return
			}
			log.Printf("Message failed (attempt %d/5), nacking for retry: %v", meta.NumDelivered, err)
			msg.Nak()
			return
		}
		log.Println("Message successfully processed and acked.")
		msg.Ack()
	})
	if err != nil {
		log.Fatalf("failed to start consumer: %v", err)
	}
	defer cc.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down consumer")
}
