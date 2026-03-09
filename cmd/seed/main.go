package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"

	"petpipeline/pets"
)

var names = []string{
	"Ogai", "Connie", "Miso", "Biscuit", "Luna", "Pickles", "Cleo", "Bear",
	"Noodle", "Rosie", "Mochi", "Ziggy", "Pepper", "Finn", "Olive", "Scout",
	"Hazel", "Gus", "Poppy", "Jasper", "Willow", "Toby", "Daisy", "Rex",
	"Maple", "Chester", "Coco", "Murphy", "Sage", "Bandit", "Lola", "Monty",
	"Beau", "Clover", "Felix", "Stella", "Mango", "Hugo", "Zelda", "Otis",
	"Peanut", "Ruby", "Atlas", "Winnie", "Archie", "Pip", "Indie", "Bruno",
	"Kiwi", "Beans",
}

var dogBreeds = []string{
	"Australian Shepherd", "Border Collie", "Golden Retriever", "Dachshund",
	"Bernese Mountain Dog", "Beagle", "Labrador Retriever", "Poodle",
	"French Bulldog", "German Shepherd", "Husky", "Corgi", "Shiba Inu",
	"Rottweiler", "Boxer", "Dalmatian", "Great Dane", "Pomeranian",
	"Cavalier King Charles", "Whippet",
}

var catBreeds = []string{
	"Ragdoll", "Maine Coon", "Siamese", "Scottish Fold", "Persian",
	"Bengal", "British Shorthair", "Abyssinian", "Sphynx", "Russian Blue",
	"Birman", "Norwegian Forest", "Tonkinese", "Burmese", "Devon Rex",
}

type breedInfo struct {
	species string
	breed   string
	minKG   float32
	maxKG   float32
}

func allBreeds() []breedInfo {
	var breeds []breedInfo
	dogWeights := [][2]float32{
		{18, 28}, {14, 22}, {25, 35}, {5, 11}, {35, 50},
		{9, 14}, {25, 36}, {18, 30}, {8, 14}, {25, 40},
		{20, 30}, {10, 14}, {8, 12}, {35, 55}, {25, 35},
		{20, 30}, {45, 75}, {1.5, 3.5}, {5, 8}, {8, 15},
	}
	for i, b := range dogBreeds {
		breeds = append(breeds, breedInfo{"Dog", b, dogWeights[i][0], dogWeights[i][1]})
	}
	catWeights := [][2]float32{
		{4, 9}, {5, 11}, {3, 5}, {3, 6}, {3, 7},
		{4, 7}, {4, 8}, {3, 5}, {3, 5}, {3, 7},
		{3, 7}, {4, 9}, {3, 5}, {3, 6}, {2.5, 4},
	}
	for i, b := range catBreeds {
		breeds = append(breeds, breedInfo{"Cat", b, catWeights[i][0], catWeights[i][1]})
	}
	return breeds
}

func generatePets(n int) []pets.Pet {
	breeds := allBreeds()
	result := make([]pets.Pet, n)
	for i := range n {
		b := breeds[rand.Intn(len(breeds))]
		weight := b.minKG + rand.Float32()*(b.maxKG-b.minKG)
		result[i] = pets.Pet{
			Name:      fmt.Sprintf("%s-%d", names[rand.Intn(len(names))], i+1),
			Species:   b.species,
			Breed:     b.breed,
			Age:       uint8(1 + rand.Intn(15)),
			WeightKG: float32(int(weight*10)) / 10,
		}
	}
	return result
}

func main() {
	ingestURL := os.Getenv("INGEST_URL")
	if ingestURL == "" {
		ingestURL = "http://localhost:5000"
	}
	endpoint := ingestURL + "/ingest"

	allPets := generatePets(10000)
	success := 0

	for i, pet := range allPets {
		body, err := json.Marshal(pet)
		if err != nil {
			log.Fatalf("failed to marshal %q: %v", pet.Name, err)
		}

		resp, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Fatalf("failed to post %q: %v", pet.Name, err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusAccepted {
			success++
		} else {
			log.Printf("unexpected status %d for %q", resp.StatusCode, pet.Name)
		}

		if (i+1)%1000 == 0 {
			fmt.Printf("progress: %d/%d\n", i+1, len(allPets))
		}
	}

	fmt.Printf("\ndone — seeded %d/%d pets\n", success, len(allPets))
}
