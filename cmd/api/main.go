package main

import (
	"log"
	"net/http"

	"petpipeline/internal/platform"
	"petpipeline/pets"
)

func main() {
	store, err := platform.ConnectClickHouseMulti()
	if err != nil {
		log.Fatalf("ClickHouse: %v", err)
	}
	server := pets.NewPetServer(nil, store)

	log.Printf("Pet api server listening on :5001")
	log.Fatal(http.ListenAndServe(":5001", server))
}
