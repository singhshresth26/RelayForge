# RelayForge: Distributed Webhook Delivery Infrastructure

[![CI Pipeline](https://github.com/relayforge/relayforge/actions/workflows/ci.yml/badge.svg)](https://github.com/relayforge/relayforge/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/relayforge/relayforge)](https://goreportcard.com/report/github.com/relayforge/relayforge)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**RelayForge** is an enterprise-grade distributed webhook delivery infrastructure platform built in Go. It enables applications to publish events with zero-loss at-least-once delivery guarantees to external third-party consumer endpoints.

---

## 🏗️ Architecture & Core Distributed Systems Patterns

```
 [Clients / Producers]
          │ (POST /events with Idempotency-Key)
          ▼
┌───────────────────────────────────────────────┐
│              RelayForge API Server            │
│  - Chi HTTP Router & Token-Bucket Rate Limiter│
│  - Idempotency & Validation Guard             │
└───────┬───────────────────────────────┬───────┘
        │                               │
        │ ACID Transaction              ▼
        │ (Dual-Write Prevention)     [Redis 7.x]
        ▼                             - Token Bucket Rate Limiter
┌─────────────────────────────────┐   - Endpoint Cache
│          PostgreSQL 16          │
│ - events                        │
│ - outbox_events                 │
│ - endpoints & subscriptions     │
│ - deliveries & attempts         │
└───────┬─────────────────────────┘
        │
        │ SELECT ... FOR UPDATE SKIP LOCKED
        ▼
┌─────────────────────────────────┐
│     Transactional Outbox Pub    │
│ (RabbitMQ Publisher Confirms)   │
└───────┬─────────────────────────┘
        │
        ▼
┌────────────────────────────────────────────────────────┐
│                   RabbitMQ Message Broker              │
│  Exchange: relayforge.events (Topic)                   │
│    └── Queue: relayforge.events.fanout                 │
│  Exchange: relayforge.deliveries (Direct)              │
│    └── Queue: relayforge.deliveries.process            │
│  Exchange: relayforge.retries (DLX / Delay TTL)        │
│    └── Queue: relayforge.retries.wait                  │
│  Exchange: relayforge.dlq (Dead Letter Queue)          │
│    └── Queue: relayforge.deliveries.dlq                │
└───────┬────────────────────────────────────────────────┘
        │
        ├─────────────────────────────┐
        ▼                             ▼
┌──────────────────────────┐  ┌───────────────────────────────────┐
│   Event Fan-Out Engine   │  │    Delivery Worker Pool (N Goroutines)│
│ - Endpoint Subscription  │  │ - SSRF Protection (DNS Pinning)   │
│   Resolution (Exact & *) │  │ - HMAC-SHA256 Signing             │
│ - Provision Deliveries   │  │ - Connection Pooling              │
└──────────────────────────┘  │ - Jittered Exponential Backoff   │
                              └─────────────────┬─────────────────┘
                                                │
                                                ▼ HTTP POST
                                     [Customer Webhooks]
```

---

## 🚀 Key Engineering Highlights

1. **Transactional Outbox with `FOR UPDATE SKIP LOCKED`**:
   - Ingested events and outbox records are inserted within a single PostgreSQL ACID transaction, eliminating the dual-write failure mode.
   - Multiple outbox publisher instances poll concurrently using `SELECT ... FOR UPDATE SKIP LOCKED` in batches, guaranteeing lock-free, zero-contention publishing.
   - Messages are acknowledged only after RabbitMQ **Publisher Confirms**.

2. **Server-Side Request Forgery (SSRF) Hardening & DNS Pinning**:
   - Webhook delivery client features a custom socket `net.Dialer.Control` validating that resolved IP addresses are never private, loopback (`127.0.0.0/8`, `::1`), or cloud metadata (`169.254.169.254`).
   - Sockets connect directly to the verified IP, mitigating Time-Of-Check to Time-Of-Use (TOCTOU) DNS rebinding attacks.

3. **HMAC-SHA256 Signatures**:
   - Webhook requests include `X-RelayForge-Timestamp` and `X-RelayForge-Signature` (`v1=<hex>`) calculated over `${timestamp}.${raw_payload}`.
   - Endpoint secrets (`whsec_...`) are encrypted at rest with AES-256-GCM.

4. **Exponential Backoff with Full Randomized Jitter**:
   - Failed webhook deliveries retry using:
     $$T_{\text{delay}} = \min(T_{\text{max}}, T_{\text{base}} \times 2^{\text{attempt}}) + \text{rand}(0, T_{\text{base}})$$
   - Leverages RabbitMQ per-message TTL + Dead-Letter Exchanges for non-blocking delays without polling or sleep loops.
   - Exhausted retries transition to `FAILED` and route to the Dead Letter Queue (`relayforge.dlq`) for manual or automated replay.

5. **Distributed Token Bucket Rate Limiting**:
   - Redis Lua script executes atomic token checks and refills per tenant/endpoint.
   - Fails gracefully with standard HTTP 429 and `Retry-After` / `X-RateLimit-*` headers.

6. **Observability & Metrics**:
   - Prometheus metrics scraped from `/metrics`: ingestion throughput (RPS), p95/p99 HTTP latency, delivery attempt counters, outbox pending lag gauge, and DLQ depth.
   - OpenTelemetry W3C trace context header propagation (`traceparent`).

---

## ⚡ Quick Start with Docker Compose

Spin up the entire distributed infrastructure (Postgres, Redis, RabbitMQ, API server, Outbox publisher, Fan-Out processor, Delivery workers, Prometheus, and Grafana) with one command:

```bash
docker compose -f deployments/docker-compose.yml up -d --build
```

### Service Endpoints
- **RelayForge Developer Console**: [http://localhost:8080/dashboard](http://localhost:8080/dashboard)
- **API Server & Health**: [http://localhost:8080/healthz](http://localhost:8080/healthz)
- **Prometheus Metrics**: [http://localhost:8080/metrics](http://localhost:8080/metrics)
- **RabbitMQ Management**: [http://localhost:15672](http://localhost:15672) (guest / guest)
- **Grafana Dashboard**: [http://localhost:3000](http://localhost:3000) (admin / admin)

---

## 🛠️ Local Development & Testing

### 1. Build Unified Binary
```bash
make build
# Binary created at ./bin/relayforge
```

### 2. Run All Unit & Integration Tests
```bash
make test
```

### 3. CLI Subcommands
```bash
# Start API Server
./bin/relayforge api

# Start Transactional Outbox Publisher
./bin/relayforge outbox

# Start Event Fan-Out Processor
./bin/relayforge fanout

# Start Delivery Worker Pool
./bin/relayforge worker
```

---

## 📡 REST API Reference

### 1. Register Tenant & Create API Key
```bash
curl -X POST http://localhost:8080/api/v1/auth/tenants \
  -H "Content-Type: application/json" \
  -d '{"name": "Acme Corp", "slug": "acme-corp", "plan": "enterprise"}'
```

### 2. Ingest Webhook Event
```bash
curl -X POST http://localhost:8080/api/v1/apps/{app_id}/events \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer rf_live_..." \
  -H "Idempotency-Key: idemp_order_9981" \
  -d '{
    "event_type": "order.completed",
    "payload": {
      "order_id": "ord_9981",
      "amount_cents": 12500,
      "currency": "USD"
    }
  }'
```
Response (`202 Accepted`):
```json
{
  "success": true,
  "data": {
    "event_id": "019485ab-9c1e-7f32-8df2-04e3e3b1c5a9",
    "status": "ACCEPTED",
    "is_duplicate": false
  }
}
```

### 3. Replay Failed Delivery
```bash
curl -X POST http://localhost:8080/api/v1/apps/{app_id}/deliveries/{delivery_id}/replay \
  -H "Authorization: Bearer rf_live_..."
```

---

## 🧪 Load Testing with k6

Simulate 200 concurrent producers with idempotency keys and evaluate p95 latency:

```bash
k6 run tests/load/ingestion_load.js
```
Expected performance:
- Ingestion Latency: `p(95) < 50ms`, `p(99) < 120ms`
- Error Rate: `0.00%`

---

## 📜 License
MIT License. Crafted for distributed systems engineering excellence.

