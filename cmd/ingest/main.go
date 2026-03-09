package main

import (
	"log"
	"net/http"

	"petpipeline/internal/platform"
	"petpipeline/pets"
)

func main() {
	_, js, natsCleanup, err := platform.ConnectNATS()
	if err != nil {
		log.Fatalf("NATS: %v", err)
	}
	defer natsCleanup()

	store := pets.NewNatsPetStore(js)
	server := pets.NewPetServer(store, nil)

	log.Printf("Pet ingest server listening on :5000")
	log.Fatal(http.ListenAndServe(":5000", server))
}
