package pets

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

type PetServer struct {
	Writer PetWriter
	Reader PetReader
	mux    *http.ServeMux
}

func NewPetServer(writer PetWriter, reader PetReader) *PetServer {
	p := &PetServer{Writer: writer, Reader: reader}
	mux := http.NewServeMux()

	if reader != nil {
		mux.HandleFunc("GET /pets", func(w http.ResponseWriter, r *http.Request) {
			filter := PetFilter{Limit: 50}
			if s := r.URL.Query().Get("limit"); s != "" {
				if n, err := strconv.Atoi(s); err == nil {
					filter.Limit = n
				}
			}
			filter.Species = r.URL.Query().Get("species")
			filter.Breed = r.URL.Query().Get("breed")
			p.ListPets(w, r.Context(), filter)
		})
		mux.HandleFunc("GET /pets/{id}", func(w http.ResponseWriter, r *http.Request) {
			p.ShowPet(w, r.Context(), r.PathValue("id"))
		})
	}

	if writer != nil {
		mux.HandleFunc("POST /ingest", func(w http.ResponseWriter, r *http.Request) {
			p.AddPet(w, r)
		})
	}

	p.mux = mux
	return p
}

func (p *PetServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mux.ServeHTTP(w, r)
}

func (p *PetServer) ListPets(w http.ResponseWriter, ctx context.Context, filter PetFilter) {
	pets, err := p.Reader.GetAllPets(ctx, filter)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		log.Printf("ListPets: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to list pets"})
		return
	}
	if len(pets) == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "no pets found"})
		return
	}
	json.NewEncoder(w).Encode(pets)
}

func (p *PetServer) ShowPet(w http.ResponseWriter, ctx context.Context, id string) {
	pet, err := p.Reader.GetPet(ctx, id)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "pet not found"})
		return
	}
	json.NewEncoder(w).Encode(pet)
}

func (p *PetServer) AddPet(w http.ResponseWriter, r *http.Request) {
	var pet Pet
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&pet); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}

	if pet.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "pet name is required"})
		return
	}

	id, err := p.Writer.RecordPet(r.Context(), pet)
	if err != nil {
		log.Printf("AddPet: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to record pet"})
		return
	}
	pet.ID = id
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(pet)
}
