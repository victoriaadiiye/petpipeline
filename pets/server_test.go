package pets_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"petpipeline/pets"
)

type StubPetStore struct {
	pets     map[string]pets.Pet
	addCalls []pets.Pet
}

func (s *StubPetStore) GetPet(_ context.Context, id string) (pets.Pet, error) {
	pet, found := s.pets[id]
	if !found {
		return pets.Pet{}, fmt.Errorf("pet %q not found", id)
	}
	return pet, nil
}

func (s *StubPetStore) RecordPet(_ context.Context, pet pets.Pet) (string, error) {
	s.addCalls = append(s.addCalls, pet)
	s.pets[pet.Name] = pet
	return pet.Name, nil
}

func (s *StubPetStore) GetAllPets(_ context.Context, filter pets.PetFilter) ([]pets.Pet, error) {
	result := make([]pets.Pet, 0, len(s.pets))
	for _, pet := range s.pets {
		result = append(result, pet)
	}
	return result, nil
}

func TestGETPets(t *testing.T) {
	store := &StubPetStore{
		pets: map[string]pets.Pet{
			"Buddy": {Name: "Buddy", Species: "Dog", Age: 3},
			"Milo":  {Name: "Milo", Species: "Cat", Age: 5},
		},
	}
	server := pets.NewPetServer(store, store)

	tests := map[string]struct {
		petName            string
		expectedHTTPStatus int
		expectedPet        *pets.Pet
	}{
		"Returns Buddy's info": {
			petName:            "Buddy",
			expectedHTTPStatus: http.StatusOK,
			expectedPet:        &pets.Pet{Name: "Buddy", Species: "Dog", Age: 3},
		},
		"Returns Milo's info": {
			petName:            "Milo",
			expectedHTTPStatus: http.StatusOK,
			expectedPet:        &pets.Pet{Name: "Milo", Species: "Cat", Age: 5},
		},
		"Returns 404 on missing pets": {
			petName:            "Ghost",
			expectedHTTPStatus: http.StatusNotFound,
			expectedPet:        nil,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
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
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("pet mismatch (-want +got):\n%s", diff)
	}
}

func newGetPetRequest(id string) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, "/pets/"+id, nil)
	return req
}
