.PHONY: up down clean nats-logs nats-cli streams consumers pub ch-cli ch-logs ch-query \
	migrate ingest consumer api seed load \
	cluster-create cluster-delete images-push k8s-apply k8s-down k8s-up k8s-deploy k8s-logs k8s-pods

REGISTRY = k3d-petpipeline-registry.localhost:5050

# ── Docker Compose ──────────────────────────────────────────

# Start everything (NATS streams provisioned by nats-provision container)
up:
	docker compose up -d --build
	@echo ""
	@echo "NATS running at        nats://localhost:4222"
	@echo "NATS monitoring at     http://localhost:8222"
	@echo "ClickHouse HTTP at     http://localhost:8123"
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
	docker compose run --rm nats-provision nats -s nats://nats:4222

# List all streams
streams:
	docker compose run --rm nats-provision nats -s nats://nats:4222 stream ls

# List consumers (usage: make consumers STREAM=PETS_DOGS)
consumers:
	docker compose run --rm nats-provision nats -s nats://nats:4222 consumer ls $(STREAM)

# Quick publish (usage: make pub SUBJECT=pets.dogs MSG='{"name":"Rex","species":"Dog"}')
pub:
	docker compose run --rm nats-provision nats -s nats://nats:4222 pub $(SUBJECT) '$(MSG)'

# ── ClickHouse ─────────────────────────────────────────────

# Open ClickHouse CLI
ch-cli:
	docker compose exec clickhouse clickhouse-client -u dev --password dev -d app

# Tail ClickHouse logs
ch-logs:
	docker compose logs -f clickhouse

# Quick query (usage: make ch-query Q="SELECT count() FROM dogs")
ch-query:
	echo '$(Q)' | docker compose exec -T clickhouse clickhouse-client -u dev --password dev -d app

# ── App ────────────────────────────────────────────────────

# Run ClickHouse migrations
migrate:
	go run ./cmd/migrate

# Run the ingest service
ingest:
	go run ./cmd/ingest

# Run the dogs consumer (set CONSUMER_STREAM/CONSUMER_TABLE to change)
consumer:
	CONSUMER_STREAM=PETS_DOGS CONSUMER_TABLE=dogs go run ./cmd/consumer

# Run the api service
api:
	go run ./cmd/api

# Seed sample pets via the ingest service
seed:
	go run ./cmd/seed

# Run the load generator (usage: make load RATE=200 DURATION=60s CONCURRENCY=20 SPECIES=Dog)
load:
	go run ./cmd/load \
		-rate=$(or $(RATE),50) \
		-duration=$(or $(DURATION),30s) \
		-concurrency=$(or $(CONCURRENCY),10) \
		$(if $(SPECIES),-species=$(SPECIES))

# ── k3d / Kubernetes ────────────────────────────────────────

# Create a k3d cluster with a local registry and port mappings:
#   host:5000  → ingest  (NodePort 30500)
#   host:5001  → api     (NodePort 30501)
#   host:8123  → ClickHouse Play UI (NodePort 30123)
#   host:4222  → NATS    (NodePort 30422)
cluster-create:
	@echo "Stopping Docker Compose (ports must be free for k3d)..."
	docker compose down || true
	k3d registry create petpipeline-registry.localhost --port 5050 || true
	k3d cluster create petpipeline \
		--registry-use k3d-petpipeline-registry.localhost:5050 \
		--port "5000:30500@loadbalancer" \
		--port "5001:30501@loadbalancer" \
		--port "8123:30123@loadbalancer" \
		--port "4222:30422@loadbalancer"
	@echo "k3d cluster created — registry at $(REGISTRY)"

# Delete the k3d cluster (keeps the registry)
cluster-delete:
	k3d cluster delete petpipeline

# Build and push all service images to the local k3d registry
images-push:
	for svc in migrate ingest consumer api; do \
		docker build --build-arg SERVICE=$$svc -t $(REGISTRY)/petpipeline/$$svc:latest . && \
		docker push $(REGISTRY)/petpipeline/$$svc:latest; \
	done

# Apply all Kubernetes manifests (ordered by filename prefix)
k8s-apply:
	kubectl apply -f k8s/

# Delete all Kubernetes resources
k8s-down:
	kubectl delete -f k8s/ --ignore-not-found

# Full bootstrap: create cluster → build/push images → apply manifests
k8s-up: cluster-create images-push k8s-apply
	@echo ""
	@echo "k3d cluster running!"
	@echo "Ingest:          http://localhost:5000"
	@echo "API:             http://localhost:5001"
	@echo "ClickHouse UI:   http://localhost:8123/play"
	@echo "NATS:            nats://localhost:4222"

# Rebuild and redeploy a single service (usage: make k8s-deploy SVC=api)
k8s-deploy:
	docker build --build-arg SERVICE=$(SVC) -t $(REGISTRY)/petpipeline/$(SVC):latest .
	docker push $(REGISTRY)/petpipeline/$(SVC):latest
	kubectl rollout restart deployment/$(SVC) -n petpipeline

# Stream logs for a service (usage: make k8s-logs SVC=consumer-dogs)
k8s-logs:
	kubectl logs -n petpipeline -l app=$(SVC) -f --max-log-requests 5

# Show all pods in the petpipeline namespace
k8s-pods:
	kubectl get pods -n petpipeline
