package pets_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"petpipeline/pets"
)

type StubPetStore struct {
	pets     map[string]pets.Pet
	addCalls []pets.Pet
}

func (s *StubPetStore) GetPet(id string) (pets.Pet, bool) {
	pet, found := s.pets[id]
	return pet, found
}

func (s *StubPetStore) RecordPet(pet pets.Pet) (string, bool) {
	s.addCalls = append(s.addCalls, pet)
	s.pets[pet.Name] = pet
	return pet.Name, true
}

func (s *StubPetStore) GetAllPets(filter pets.PetFilter) ([]pets.Pet, int) {
	result := make([]pets.Pet, 0, len(s.pets))
	for _, pet := range s.pets {
		result = append(result, pet)
	}
	return result, len(result)
}

func TestGETPets(t *testing.T) {
	store := &StubPetStore{
		pets: map[string]pets.Pet{
			"Buddy": {Name: "Buddy", Species: "Dog", Age: 3},
			"Milo":  {Name: "Milo", Species: "Cat", Age: 5},
		},
	}
	server := pets.NewPetServer(store, store)

	tests := []struct {
		name               string
		petName            string
		expectedHTTPStatus int
		expectedPet        *pets.Pet
	}{
		{
			name:               "Returns Buddy's info",
			petName:            "Buddy",
			expectedHTTPStatus: http.StatusOK,
			expectedPet:        &pets.Pet{Name: "Buddy", Species: "Dog", Age: 3},
		},
		{
			name:               "Returns Milo's info",
			petName:            "Milo",
			expectedHTTPStatus: http.StatusOK,
			expectedPet:        &pets.Pet{Name: "Milo", Species: "Cat", Age: 5},
		},
		{
			name:               "Returns 404 on missing pets",
			petName:            "Ghost",
			expectedHTTPStatus: http.StatusNotFound,
			expectedPet:        nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := newGetPetRequest(tt.petName)
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			assertStatus(t, response.Code, tt.expectedHTTPStatus)

			if tt.expectedPet != nil {
				var got pets.Pet
				json.NewDecoder(response.Body).Decode(&got)
				assertPet(t, got, *tt.expectedPet)
			}
		})
	}
}

func TestPOSTPet(t *testing.T) {
	store := &StubPetStore{pets: map[string]pets.Pet{}}
	server := pets.NewPetServer(store, store)

	t.Run("it records a pet on POST with JSON body", func(t *testing.T) {
		petJSON := `{"name":"Buddy","species":"Dog","age":3}`
		request, _ := http.NewRequest(http.MethodPost, "/ingest", strings.NewReader(petJSON))
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		assertStatus(t, response.Code, http.StatusAccepted)

		if len(store.addCalls) != 1 {
			t.Fatalf("got %d calls to RecordPet want %d", len(store.addCalls), 1)
		}

		assertPet(t, store.addCalls[0], pets.Pet{Name: "Buddy", Species: "Dog", Age: 3})
	})

	t.Run("it returns 400 on invalid JSON", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodPost, "/ingest", strings.NewReader("not json"))
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		assertStatus(t, response.Code, http.StatusBadRequest)
	})

	t.Run("it returns 400 when name is missing", func(t *testing.T) {
		petJSON := `{"species":"Dog","age":3}`
		request, _ := http.NewRequest(http.MethodPost, "/ingest", strings.NewReader(petJSON))
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		assertStatus(t, response.Code, http.StatusBadRequest)
	})
}

func TestListPets(t *testing.T) {
	store := &StubPetStore{
		pets: map[string]pets.Pet{
			"Buddy": {Name: "Buddy", Species: "Dog", Age: 3},
		},
	}
	server := pets.NewPetServer(store, store)

	request, _ := http.NewRequest(http.MethodGet, "/pets", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	assertStatus(t, response.Code, http.StatusOK)

	var got []pets.Pet
	json.NewDecoder(response.Body).Decode(&got)

	if len(got) != 1 {
		t.Errorf("expected 1 pet, got %d", len(got))
	}
}

func assertStatus(t testing.TB, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("did not get correct status, got %d, want %d", got, want)
	}
}

func assertPet(t testing.TB, got, want pets.Pet) {
	t.Helper()
	if got != want {
		t.Errorf("pet mismatch, got %+v want %+v", got, want)
	}
}

func newGetPetRequest(id string) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, "/pets/"+id, nil)
	return req
}
