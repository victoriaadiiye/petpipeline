package main

import (
	"log"
	"net/http"

	"petpipeline/internal/infra"
	"petpipeline/pets"
)

func main() {
	_, js, natsCleanup := infra.ConnectNATS()
	defer natsCleanup()

	store := pets.NewNatsPetStore(js)
	server := pets.NewPetServer(store, nil)

	log.Printf("Pet ingest server listening on :5000")
	log.Fatal(http.ListenAndServe(":5000", server))
}
