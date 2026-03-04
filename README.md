# petpipeline

A Go pipeline that ingests pet data over HTTP, buffers it through NATS JetStream, persists it to ClickHouse, and serves it via a query API.

```
HTTP POST /ingest ──▶ Go Ingestion Service ──▶ NATS JetStream ──▶ Consumer ──▶ ClickHouse
                                                                                    │   
                                                                    GET /pets ◀──── API
                                                                GET /pets/:id ◀──── API
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| **ingest** | 5000 | Accepts pet JSON via `POST /ingest`, publishes to NATS |
| **consumer** | — | Reads from NATS stream, writes to ClickHouse. Retries up to 5 times per message. |
| **api** | 5001 | Serves `GET /pets` and `GET /pets/{id}` from ClickHouse |
| **migrate** | — | Runs ClickHouse schema migrations via golang-migrate |
| **seed** | — | Generates 10,000 random pets and posts them to the ingest service |

## Quick start

```bash
# Start everything (NATS, ClickHouse, migrations, all services)
make up

# Seed 10,000 test pets
make seed

# Query
curl http://localhost:5001/pets?species=Dog&limit=5
curl http://localhost:5001/pets/{id}
```

## Running locally (without Docker)

```bash
# Start infrastructure only
docker compose up -d nats clickhouse
make migrate

# Run services individually
make ingest
make consumer
make api

# Seed data
make seed
```

## Makefile targets

```
up              Start all containers and provision NATS stream
down            Stop containers
clean           Stop containers and wipe all data volumes

migrate         Run ClickHouse migrations
ingest          Run ingest service locally
consumer        Run consumer service locally
api             Run API service locally
seed            Generate 10,000 test pets via ingest

nats-logs       Tail NATS server logs
nats-cli        Open NATS CLI shell
streams         List NATS streams
consumers       List consumers (usage: make consumers STREAM=PETS)
pub             Publish a message (usage: make pub SUBJECT=pets.test MSG='{"hello":"world"}')

ch-cli          Open ClickHouse client
ch-logs         Tail ClickHouse logs
ch-query        Run a query (usage: make ch-query Q="SELECT count() FROM pets")
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

Query parameters: `species`, `breed`, `limit`.

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
  ingest/       HTTP server → NATS publisher
  consumer/     NATS subscriber → ClickHouse writer
  api/          ClickHouse reader → HTTP server
  migrate/      Schema migrations runner
  seed/         Test data generator
pets/
  pet.go                    Domain types and interfaces (Pet, PetWriter, PetReader)
  server.go                 HTTP handlers
  clickhouse_pet_store.go   ClickHouse implementation of PetWriter/PetReader
  nats_pet_store.go         NATS implementation of PetWriter
  *_test.go                 Unit and integration tests
db/
  migrations.go             Embedded migration files (embed.FS)
  migrations/               SQL migration files
clickhouse/                 ClickHouse config and init scripts
```

## Environment variables

| Variable | Default | Used by |
|----------|---------|---------|
| `NATS_URL` | `nats://localhost:4222` | ingest, consumer |
| `CLICKHOUSE_ADDR` | `127.0.0.1:9000` | api, consumer, migrate |
| `INGEST_URL` | `http://localhost:5000` | seed |

## Monitoring

- NATS health: http://localhost:8222/healthz
- NATS server info: http://localhost:8222/varz
- JetStream info: http://localhost:8222/jsz
- ClickHouse Play UI: http://localhost:8123/play

## Tech stack

- **Go 1.24**
- **NATS JetStream** — message streaming with durable consumers
- **ClickHouse** — columnar analytics database (MergeTree engine)
- **golang-migrate** — schema migrations with embedded SQL files
- **Docker Compose** — local orchestration with health checks and service dependencies
