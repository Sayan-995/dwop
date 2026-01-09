# DWOP — Distributed Workflow Orchestrator

A workflow engine that transforms Python DSL into fault-tolerant, distributed execution pipelines.

DWOP combines transactional outbox patterns, message queuing, and Kubernetes job orchestration to deliver reliable, observable workflow execution with deterministic outputs and comprehensive failure handling.

---

## System Architecture
```mermaid
flowchart TD
    Client[Client] -->|Upload DSL| API[HTTP API]
    
    API -->|Parse & Store| DB[(Supabase<br/>DB + Storage)]
    API -->|Write Ready Tasks| Outbox[(Outbox Table)]
    
    Claimer[Claimer Loop] -->|Claim Events| Outbox
    Claimer -->|Publish| RMQ[RabbitMQ]
    
    RMQ -->|Deliver| Consumer[Consumers]
    Consumer -->|Create| Jobs[K8s Jobs]
    
    Jobs -->|Execute| Worker[Python Worker]
    Worker <-->|Signed URLs| DB
    
    Observer -->|Success: Enqueue Next<br/>Failure: Retry| Outbox
    Observer[Observer] -->|Watch| Jobs
    
    style API fill:#4A90E2
    style Claimer fill:#4A90E2
    style Consumer fill:#4A90E2
    style Observer fill:#4A90E2
    style RMQ fill:#F5A623
    style Jobs fill:#BD10E0
    style Worker fill:#BD10E0
```

## Overview

**Core Components:**

- **Orchestrator (Go):** Single-process control plane running HTTP API, outbox claimer, RabbitMQ consumers, and Kubernetes job observer
- **Worker (Python container):** Executes task code in isolated Kubernetes Jobs with signed-URL-based I/O
- **State Layer:** Supabase (PostgreSQL + Storage) for workflow metadata and artifact persistence
- **Message Bus:** RabbitMQ for reliable task distribution

**Execution Flow:**

1. Client submits workflow DSL → Backend parses into task DAG
2. Workflow and tasks persisted; ready tasks written to transactional outbox
3. Outbox claimer claims events via database RPC, publishes to RabbitMQ
4. Consumers create Kubernetes Jobs for claimed tasks
5. Job observer watches for completion, marks success/failure, enqueues dependent tasks
6. Workers materialize predecessor outputs, execute task code, upload results to deterministic storage paths

---

## Workflow DSL

Tasks are defined using a Python-like syntax with explicit dependency declarations:

```python
# sample.workflow
fun extract_data():
    import requests
    data = requests.get("https://api.example.com/data").json()
    print(data)

fun transform(raw:extract_data):
    # 'raw' is the filename containing extract_data's output
    with open("raw") as f:
        data = f.read()
    transformed = data.upper()
    print(transformed)

fun load(processed:transform):
    with open("processed") as f:
        print(f"Loading: {f.read()}")
```

**Dependency Semantics:**

- `fun b(x:a):` declares `b` depends on `a`
- Task `a`'s output downloads to file `x` before `b` executes
- Workers execute function bodies as standalone Python scripts

**Storage Model:**

Outputs are stored deterministically at:
```
Task_Output/<workflowId>/<taskName>/output.txt
```

This enables:
- Idempotent task execution (reruns write to same location)
- Simple output resolution for dependent tasks
- Straightforward artifact retrieval for debugging

---

## Fault Tolerance & Reliability Design

### 1. Transactional Outbox Pattern

Workflow creation atomically writes outbox rows in the same transaction as workflow metadata. Publishing to RabbitMQ happens asynchronously via dedicated claimer process.

**Why:** Eliminates "database committed, queue publish failed" split-brain scenarios. Makes retries safe and explicit.

### 2. Optimistic Concurrency via Database RPC

Outbox rows are claimed through `claim_outbox_events(claimer_id, batch_size)` stored procedure with optimistic locking.

**Why:** Enables horizontal scaling of claimers without double-processing. Provides natural batching and backpressure control.

### 3. Idempotent Job Creation

RabbitMQ messages are treated as at-least-once delivery. Consumers use database upserts (`ON CONFLICT DO NOTHING`) to ensure exactly-one Job per task run.

**Why:** Duplicate messages don't create duplicate Kubernetes Jobs. Consumer acknowledges only after successful Job creation.

### 4. Orchestrator-Controlled Retry Logic

Kubernetes Jobs run with `BackoffLimit=0`. Retry decisions live in orchestrator state via `increase_attempt(run_id, error_msg)`.

**Why:** Single source of truth for retry counts and failure diagnostics. Prevents Kubernetes from making retry decisions based on pod-level transient failures.

### 5. Watch + Periodic Reconciliation

Job observer uses Kubernetes watch API with periodic full-state reconciliation loop.

**Why:** Recovers from watch connection drops and missed events. Handles orchestrator restarts gracefully.

### 6. Terminal State Deduplication

Observer tracks processed terminal states in memory and deletes Jobs after final handling.

**Why:** Prevents reprocessing on watch event replay. Avoids "flapping" between terminal states during cleanup.

### 7. Channel Pool Failure Isolation

RabbitMQ publisher maintains channel pool. Channels that encounter errors are closed and discarded, never returned to pool.

**Why:** Prevents poisoning connection pool with broken channels. Stabilizes publishing under transient broker issues.

### 8. Zero-Credential Worker Design

Workers operate entirely on time-limited signed URLs for:
- Task code download
- Requirements.txt download  
- Predecessor output downloads
- Result upload

**Why:** Eliminates credential management in pods. Reduces blast radius of container compromise.

---

## Local Development

### Prerequisites

- Go 1.21+
- Kubernetes cluster (kind/minikube/GKE)
- RabbitMQ instance
- Supabase project

### Configuration

Create `.env` (gitignored):

```bash
SUPABASE_PROJECT_URL=https://xxxxx.supabase.co
SUPABASE_SERVICE_KEY=eyJhbG...
RABBITMQ_CONNECTION_URL=amqp://guest:guest@localhost:5672/
DWOP_IMAGE=dwop-pyworker:dev
DWOP_NAMESPACE=default  # optional
DWOP_PORT=8080          # optional
KUBECONFIG=/path/to/kubeconfig  # optional
```

### Start Orchestrator

```bash
go run ./cmd/backend/main.go
```

Backend starts HTTP server (port 8080), outbox claimer, RabbitMQ consumers, and Kubernetes job observer.

### Build Worker Image

```bash
docker build -t dwop-pyworker:dev -f cmd/pyworker/Dockerfile cmd/pyworker
```

For kind clusters:
```bash
kind load docker-image dwop-pyworker:dev
```

### Submit Workflows

Via provided Streamlit UI:
```bash
pip install -r ui/requirements.txt
streamlit run ui/streamlit_app.py
```

Or directly via API:
```bash
curl -X POST http://localhost:8080/workflows \
  -F "workflow=@sample.workflow" \
  -F "requirements=@requirements.txt"
```

---

## Debugging Common Issues

### Task fails with non-zero exit code

Observer automatically captures pod logs in the run's error field. Check via API:
```bash
curl http://localhost:8080/workflows/{id}/runs/{runId}
```

Common causes:
- Missing predecessor output (verify storage bucket path)
- Incorrect input file reference (must match parameter name in DSL)
- Python dependencies missing from requirements.txt

### Storage signed URLs failing

Executor normalizes all Supabase Storage URLs to absolute paths. If seeing relative URLs:
- Verify `SUPABASE_PROJECT_URL` is fully qualified (includes `https://`)
- Check Storage bucket `Task_Output` exists and has appropriate policies

### Worker code changes not reflected

Kubernetes Jobs use `imagePullPolicy: Never` in local clusters:
1. Rebuild image: `docker build -t dwop-pyworker:dev ...`
2. Load into cluster: `kind load docker-image dwop-pyworker:dev`
3. Delete existing Jobs: `kubectl delete job -l app=dwop`

### Outbox events not processing

Check claimer logs for RPC errors:
- Verify `claim_outbox_events` stored procedure exists
- Ensure database user has EXECUTE permission
- Confirm RabbitMQ connection is healthy (check consumer count)

---
