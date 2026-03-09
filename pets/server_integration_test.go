package pets_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"petpipeline/pets"
	"strings"
	"testing"
)

func TestRecordingPetsAndRetrievingThem(t *testing.T) {
	db := connectClickHouse(t)
	db.Exec(context.Background(), "TRUNCATE TABLE pets")
	store := pets.NewClickHousePetStore(db, "pets")
	server := pets.NewPetServer(store, store)

	ogaiJSON := `{"name":"ogai","species":"dog","breed":"aussie","age":3,"weight_kg":12}`
	connieJSON := `{"name":"connie","species":"dog","breed":"Collie","age":4,"weight_kg":19}`

	// Add two pets and capture the first pet's ID from the response
	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/ingest", strings.NewReader(ogaiJSON))
	server.ServeHTTP(rec, req)
	assertStatus(t, rec.Code, http.StatusAccepted)

	var ogai pets.Pet
	json.NewDecoder(rec.Body).Decode(&ogai)

	rec = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/ingest", strings.NewReader(connieJSON))
	server.ServeHTTP(rec, req)
	assertStatus(t, rec.Code, http.StatusAccepted)

	// Retrieve ogai by UUID
	response := httptest.NewRecorder()
	server.ServeHTTP(response, newGetPetRequest(ogai.ID))
	assertStatus(t, response.Code, http.StatusOK)

	var got pets.Pet
	json.NewDecoder(response.Body).Decode(&got)
	assertPet(t, got, pets.Pet{ID: ogai.ID, Name: "ogai", Species: "dog", Breed: "aussie", Age: 3, WeightKG: 12})

	// List all pets
	response = httptest.NewRecorder()
	listReq, _ := http.NewRequest(http.MethodGet, "/pets", nil)
	server.ServeHTTP(response, listReq)
	assertStatus(t, response.Code, http.StatusOK)

	var allPets []pets.Pet
	json.NewDecoder(response.Body).Decode(&allPets)

	if len(allPets) != 2 {
		t.Errorf("expected 2 pets, got %d", len(allPets))
	}
}
