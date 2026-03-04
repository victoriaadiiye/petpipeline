package main

import (
	"log"
	"net/http"

	"petpipeline/internal/infra"
	"petpipeline/pets"
)

func main() {
	store := infra.ConnectClickHouse()
	server := pets.NewPetServer(nil, store)

	log.Printf("Pet api server listening on :5001")
	log.Fatal(http.ListenAndServe(":5001", server))
}
