package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

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

func randomPet(breeds []breedInfo, filterSpecies string) pets.Pet {
	filtered := breeds
	if filterSpecies != "" {
		filtered = filtered[:0]
		for _, b := range breeds {
			if b.species == filterSpecies {
				filtered = append(filtered, b)
			}
		}
	}
	b := filtered[rand.Intn(len(filtered))]
	weight := b.minKG + rand.Float32()*(b.maxKG-b.minKG)
	return pets.Pet{
		Name:     names[rand.Intn(len(names))],
		Species:  b.species,
		Breed:    b.breed,
		Age:      uint8(1 + rand.Intn(15)),
		WeightKG: float32(int(weight*10)) / 10,
	}
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)) * p / 100)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	url := flag.String("url", getenv("INGEST_URL", "http://localhost:5000"), "ingest service base URL")
	rps := flag.Float64("rate", 50, "target requests per second (0 = unlimited)")
	dur := flag.Duration("duration", 30*time.Second, "test duration (0 = run until Ctrl-C)")
	concurrency := flag.Int("concurrency", 10, "number of concurrent workers")
	species := flag.String("species", "", "species to generate: Dog, Cat, or empty for random")
	flag.Parse()

	endpoint := *url + "/ingest"
	breeds := allBreeds()

	ctx, cancel := context.WithCancel(context.Background())
	if *dur > 0 {
		ctx, cancel = context.WithTimeout(ctx, *dur)
	}
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sig:
			cancel()
		case <-ctx.Done():
		}
	}()

	var (
		sent    atomic.Int64
		success atomic.Int64
		failed  atomic.Int64
		mu      sync.Mutex
		lats    []time.Duration
	)

	record := func(lat time.Duration, ok bool) {
		sent.Add(1)
		if ok {
			success.Add(1)
		} else {
			failed.Add(1)
		}
		mu.Lock()
		lats = append(lats, lat)
		mu.Unlock()
	}

	// Rate limiter: batch tokens to avoid sub-millisecond tickers at high rates.
	tokens := make(chan struct{}, *concurrency*2)
	go func() {
		if *rps <= 0 {
			for {
				select {
				case tokens <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
		}
		interval := time.Duration(float64(time.Second) / *rps)
		batchSize := 1
		if interval < time.Millisecond {
			batchSize = int(math.Ceil(*rps / 1000))
			interval = time.Millisecond
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				for i := 0; i < batchSize; i++ {
					select {
					case tokens <- struct{}{}:
					default: // drop token if workers can't keep up
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Progress reporter.
	start := time.Now()
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case t := <-ticker.C:
				elapsed := t.Sub(start).Seconds()
				n := sent.Load()
				mu.Lock()
				latsCopy := make([]time.Duration, len(lats))
				copy(latsCopy, lats)
				mu.Unlock()
				sort.Slice(latsCopy, func(i, j int) bool { return latsCopy[i] < latsCopy[j] })
				fmt.Printf("[%5.0fs] sent=%-6d ok=%-5d err=%-4d rps=%6.1f  p50=%v p95=%v p99=%v\n",
					elapsed, n, success.Load(), failed.Load(),
					float64(n)/elapsed,
					percentile(latsCopy, 50).Round(time.Millisecond),
					percentile(latsCopy, 95).Round(time.Millisecond),
					percentile(latsCopy, 99).Round(time.Millisecond),
				)
			case <-ctx.Done():
				return
			}
		}
	}()

	client := &http.Client{Timeout: 10 * time.Second}

	// Worker pool.
	var wg sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-tokens:
				case <-ctx.Done():
					return
				}
				pet := randomPet(breeds, *species)
				body, _ := json.Marshal(pet)

				t0 := time.Now()
				resp, err := client.Post(endpoint, "application/json", bytes.NewReader(body))
				lat := time.Since(t0)

				ok := err == nil && resp.StatusCode == http.StatusAccepted
				if err == nil {
					resp.Body.Close()
				}
				record(lat, ok)
			}
		}()
	}

	wg.Wait()

	// Final report.
	elapsed := time.Since(start)
	n := sent.Load()
	ok := success.Load()

	mu.Lock()
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	mu.Unlock()

	errPct := 0.0
	if n > 0 {
		errPct = 100 * float64(failed.Load()) / float64(n)
	}

	fmt.Printf("\n=== load test complete ===\n")
	fmt.Printf("duration:     %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("requests:     %d\n", n)
	fmt.Printf("success:      %d (%.1f%%)\n", ok, 100*float64(ok)/math.Max(float64(n), 1))
	fmt.Printf("errors:       %d (%.1f%%)\n", failed.Load(), errPct)
	fmt.Printf("throughput:   %.1f req/s\n", float64(n)/elapsed.Seconds())
	fmt.Printf("latency p50:  %v\n", percentile(lats, 50).Round(time.Millisecond))
	fmt.Printf("latency p95:  %v\n", percentile(lats, 95).Round(time.Millisecond))
	fmt.Printf("latency p99:  %v\n", percentile(lats, 99).Round(time.Millisecond))
}
