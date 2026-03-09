# petpipeline

A Go pipeline that ingests pet data over HTTP, buffers it through NATS JetStream, persists it to ClickHouse, and serves it via a query API.

```
HTTP POST /ingest ──▶ Go Ingestion Service ──▶ NATS JetStream (PETS_DOGS / PETS_CATS)
                                                      │                │
                                               consumer-dogs    consumer-cats
                                                      │                │
                                                  ClickHouse dogs   ClickHouse cats
                                                          └──────┬───────┘
                                                         GET /pets ◀── API
                                                     GET /pets/:id ◀── API
```

The ingest service routes each pet to a species-specific NATS stream based on the `species` field. Two independent consumers drain those streams into separate ClickHouse tables. The API fans out reads across both tables, merging results transparently.

## Services

| Service | Port | Description |
|---------|------|-------------|
| **ingest** | 5000 | Accepts pet JSON via `POST /ingest`, routes to `pets.dogs` or `pets.cats` NATS subject |
| **consumer-dogs** | — | Drains `PETS_DOGS` stream into the `dogs` ClickHouse table. Retries up to 5× per message. |
| **consumer-cats** | — | Drains `PETS_CATS` stream into the `cats` ClickHouse table. Retries up to 5× per message. |
| **api** | 5001 | Serves `GET /pets` and `GET /pets/{id}` — fans out across `dogs` and `cats` tables |
| **migrate** | — | Runs ClickHouse schema migrations (creates `pets`, `dogs`, `cats` tables) |
| **seed** | — | Generates 10,000 random pets and posts them to the ingest service |
| **load** | — | Configurable load generator with concurrency, rate control, and latency reporting |

## Quick start (Docker Compose)

```bash
# Start everything (NATS, ClickHouse, migrations, all services)
make up

# Seed 10,000 test pets
make seed

# Or run a load test instead
make load RATE=200 DURATION=60s

# Query
curl "http://localhost:5001/pets?species=Dog&limit=5"
curl "http://localhost:5001/pets/{id}"
```

## Quick start (k3d / Kubernetes)

Requires [k3d](https://k3d.io), `kubectl`, and Docker. Install on macOS:

```bash
brew install k3d kubectl
```

k3d uses the same host ports as Docker Compose (5000, 5001, 8123, 4222) so the two stacks can't run simultaneously. `make cluster-create` will stop Docker Compose automatically.

```bash
# 1. Create the k3d cluster and local image registry (run once)
make cluster-create

# 2. Build and push service images to the local registry
make images-push

# 3. Deploy all manifests
make k8s-apply

# Watch pods come up (NATS and ClickHouse take ~30s to become ready)
make k8s-pods

# Same endpoints — k3d forwards host ports to NodePorts
curl "http://localhost:5001/pets?species=Cat&limit=5"

# Stream logs for a service
make k8s-logs SVC=consumer-dogs

# Rebuild and redeploy a single service after a code change
make k8s-deploy SVC=api

# Tear down
make cluster-delete
```

`make k8s-up` runs steps 1–3 in one shot if you prefer.

## Running locally (without Docker)

```bash
# Start infrastructure only
docker compose up -d nats clickhouse
make migrate

# Run services (consumer is configured via env vars)
make ingest
make consumer                                        # defaults to PETS_DOGS / dogs table
CONSUMER_STREAM=PETS_CATS CONSUMER_TABLE=cats \
  go run ./cmd/consumer                              # second consumer for cats
make api

# Generate load
make load RATE=100 DURATION=30s CONCURRENCY=20 SPECIES=Dog
```

## Load generator

```bash
# 50 req/s for 30 seconds (default)
make load

# 500 req/s, cats only, 2 minutes, 50 workers
make load RATE=500 DURATION=2m CONCURRENCY=50 SPECIES=Cat

# Unlimited rate until Ctrl-C
go run ./cmd/load -rate=0 -duration=0 -concurrency=50
```

Progress is printed every 5 seconds:

```
[  5s] sent=250   ok=249   err=1    rps=  50.0  p50=8ms p95=31ms p99=67ms
[ 10s] sent=500   ok=498   err=2    rps=  50.0  p50=7ms p95=28ms p99=61ms

=== load test complete ===
duration:     30.001s
requests:     1500
success:      1497 (99.8%)
errors:       3 (0.2%)
throughput:   50.0 req/s
latency p50:  8ms
latency p95:  30ms
latency p99:  64ms
```

## Makefile targets

```
# Docker Compose
up              Start all containers and provision NATS streams
down            Stop containers
clean           Stop containers and wipe all data volumes

# App (run locally)
migrate         Run ClickHouse migrations
ingest          Run ingest service
consumer        Run dogs consumer (CONSUMER_STREAM=PETS_DOGS CONSUMER_TABLE=dogs)
api             Run API service
seed            Generate 10,000 test pets via ingest
load            Run load generator (RATE, DURATION, CONCURRENCY, SPECIES)

# NATS
nats-logs       Tail NATS server logs
nats-cli        Open NATS CLI shell
streams         List NATS streams
consumers       List consumers (usage: make consumers STREAM=PETS_DOGS)
pub             Publish a message (usage: make pub SUBJECT=pets.dogs MSG='{"name":"Rex","species":"Dog"}')

# ClickHouse
ch-cli          Open ClickHouse client
ch-logs         Tail ClickHouse logs
ch-query        Run a query (usage: make ch-query Q="SELECT count() FROM dogs")

# k3d / Kubernetes
cluster-create  Create k3d cluster and local registry
cluster-delete  Delete k3d cluster
images-push     Build and push all service images to local registry
k8s-apply       Apply all Kubernetes manifests
k8s-down        Delete all Kubernetes resources
k8s-up          Full bootstrap: cluster + images + manifests
k8s-deploy      Rebuild and redeploy one service (SVC=api)
k8s-logs        Stream logs for a service (SVC=consumer-dogs)
k8s-pods        Show all pods in the petpipeline namespace
```

## API

### `POST /ingest`

```bash
curl -X POST http://localhost:5000/ingest \
  -H "Content-Type: application/json" \
  -d '{"name":"Miso","species":"Cat","breed":"Ragdoll","age":3,"weight_kg":5.2}'
```

Returns `202 Accepted` with the pet JSON including the assigned `id`.

### `GET /pets`

Query parameters: `species`, `breed`, `limit` (default 50). When `species` is specified only that table is queried; otherwise both `dogs` and `cats` are merged.

```bash
curl "http://localhost:5001/pets?species=Cat&breed=Ragdoll&limit=10"
```

### `GET /pets/{id}`

```bash
curl http://localhost:5001/pets/550e8400-e29b-41d4-a716-446655440000
```

## Project structure

```
cmd/
  ingest/       HTTP server → NATS publisher (routes by species)
  consumer/     NATS subscriber → ClickHouse writer (stream/table via env vars)
  api/          ClickHouse reader → HTTP server (multi-table fan-out)
  migrate/      Schema migrations runner
  seed/         Test data generator (10k pets, sequential)
  load/         Configurable load generator (concurrent, rate-limited)
pets/
  pet.go                    Domain types and interfaces (Pet, PetWriter, PetReader)
  server.go                 HTTP handlers
  clickhouse_pet_store.go   ClickHouse store (table-parameterised) + MultiPetReader
  nats_pet_store.go         NATS store (routes to species-specific subject)
  *_test.go                 Unit and integration tests
db/
  migrations.go             Embedded migration files (embed.FS)
  migrations/               SQL migration files (pets, dogs, cats tables)
k8s/                        Kubernetes manifests for k3d deployment
internal/platform/
  clickhouse.go             ConnectClickHouse(table) + ConnectClickHouseMulti()
  nats.go                   ConnectNATS()
```

## Environment variables

| Variable | Default | Used by |
|----------|---------|---------|
| `NATS_URL` | `nats://localhost:4222` | ingest, consumer |
| `CLICKHOUSE_ADDR` | `127.0.0.1:9000` | api, consumer, migrate |
| `INGEST_URL` | `http://localhost:5000` | seed, load |
| `CONSUMER_STREAM` | `PETS_DOGS` | consumer |
| `CONSUMER_TABLE` | `dogs` | consumer |
| `CONSUMER_NAME` | `<stream>-consumer` | consumer |

## NATS streams

| Stream | Subject | Consumer | Table |
|--------|---------|----------|-------|
| `PETS_DOGS` | `pets.dogs` | `consumer-dogs` | `dogs` |
| `PETS_CATS` | `pets.cats` | `consumer-cats` | `cats` |

## Monitoring

- NATS health: http://localhost:8222/healthz
- NATS server info: http://localhost:8222/varz
- JetStream info: http://localhost:8222/jsz
- ClickHouse Play UI: http://localhost:8123/play

## Tech stack

- **Go 1.24**
- **NATS JetStream** — per-species streams with durable consumers
- **ClickHouse** — columnar analytics database (MergeTree engine, per-species tables)
- **golang-migrate** — schema migrations with embedded SQL files
- **Docker Compose** — local orchestration
- **k3d** — local Kubernetes (k3s in Docker) for deployment practice
