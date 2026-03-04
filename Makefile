.PHONY: up down clean nats-logs nats-cli streams consumers pub ch-cli ch-logs ch-query migrate ingest consumer seed

# ── Infrastructure ─────────────────────────────────────────

# Start everything (NATS + ClickHouse) and provision streams
up:
	docker compose up -d --build
	nats stream add PETS \
		--subjects "pets.>" \
		--storage file \
		--retention limits \
		--defaults \
		--server nats://localhost:4222 || true
	@echo ""
	@echo "NATS running at        nats://localhost:4222"
	@echo "NATS monitoring at     http://localhost:8222"
	@echo "ClickHouse HTTP at     http://localhost:8123"
	@echo "ClickHouse native at   localhost:9000"
	@echo "ClickHouse Play UI at  http://localhost:8123/play"


# Stop everything
down:
	docker compose down

# Stop and wipe all data volumes
clean:
	docker compose down -v

# ── NATS ───────────────────────────────────────────────────

# Tail NATS server logs
nats-logs:
	docker compose logs -f nats

# Open a NATS CLI shell
nats-cli:
	docker compose exec nats-box nats -s nats://nats:4222

# List all streams
streams:
	docker compose exec nats-box nats -s nats://nats:4222 stream ls

# List consumers (usage: make consumers STREAM=EVENTS)
consumers:
	docker compose exec nats-box nats -s nats://nats:4222 consumer ls $(STREAM)

# Quick publish (usage: make pub SUBJECT=events.test MSG='{"hello":"world"}')
pub:
	docker compose exec nats-box nats -s nats://nats:4222 pub $(SUBJECT) '$(MSG)'

# ── ClickHouse ─────────────────────────────────────────────

# Open ClickHouse CLI
ch-cli:
	docker compose exec clickhouse clickhouse-client -u dev --password dev -d app

# Tail ClickHouse logs
ch-logs:
	docker compose logs -f clickhouse

# Quick query (usage: make ch-query Q="SELECT count() FROM app.events")
ch-query:
	echo '$(Q)' | docker compose exec -T clickhouse clickhouse-client -u dev --password dev -d app

# ── App ────────────────────────────────────────────────────

# Run ClickHouse migrations
migrate:
	go run ./cmd/migrate

# Run the ingest service
ingest:
	go run ./cmd/ingest

# Run the consumer service
consumer:
	go run ./cmd/consumer

# Run the api service
api:
	go run ./cmd/api

# Seed sample pets via the ingest service
seed:
	go run ./cmd/seed
