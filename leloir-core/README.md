# Leloir Control Plane

This repo contains the three binaries that make up the Leloir control plane:

- **`leloir-controlplane`** — main service: API, routing, orchestration, audit, CRD reconciler
- **`leloir-mcp-gateway`** — multi-transport MCP translator with per-tenant scoping
- **`leloir-webhook-receiver`** — Alertmanager-compatible webhook ingester (optional)

## Quick start

```bash
# Build all three binaries
make build

# Run the control plane with an in-memory store (M0 PoC)
./bin/leloir-controlplane --config=examples/config.local.yaml
```

## Repo layout

```
leloir/
├── cmd/
│   ├── leloir-controlplane/       Main binary
│   ├── leloir-mcp-gateway/        MCP Gateway binary
│   └── leloir-webhook-receiver/   Webhook ingester binary
│
├── api/
│   └── v1alpha1/                  Kubernetes CRD Go types
│
├── internal/
│   ├── config/                    YAML config loader (one schema per binary)
│   │
│   ├── controlplane/              Control plane guts
│   │   ├── server/                Top-level server wiring
│   │   ├── handlers/              HTTP API + middleware
│   │   ├── routing/               Alert → AlertRoute matcher
│   │   ├── registry/              Live agent registry
│   │   ├── orchestrator/          Investigation lifecycle (the ❤)
│   │   ├── stream/                SSE fan-out broker
│   │   └── audit/                 Tamper-evident audit log
│   │
│   ├── mcpgateway/                MCP Gateway service
│   ├── webhook/                   Webhook receiver service
│   ├── llmgateway/                LLM Gateway client stubs (M3)
│   ├── store/                     Postgres / in-memory persistence
│   ├── auth/                      OIDC + multi-tenant RBAC (M2)
│   ├── budget/                    Budget enforcement helpers (M3)
│   ├── notifications/             Teams / Slack / Telegram (M1)
│   └── observability/             Logger + OpenTelemetry setup
│
├── deploy/
│   ├── helm/leloir/               Helm chart (M1)
│   └── kind/                      kind cluster config for local dev
│
├── docs/                          Architecture docs, ADRs
├── examples/                      Example configs + YAML
└── .github/                       CI workflows, issue templates
```

## What M0 needs

For M0 (the 2-week PoC), only a subset of this is used:

- `cmd/leloir-controlplane/main.go` with the `memory` store driver
- `internal/controlplane/stream` (the SSE broker — this is the Go webhook receiver from the M0 plan, simplified)
- A skeleton HolmesGPT adapter (see the companion SDK repo)

The rest is scaffolding for M1+. Most files are skeletons with `// M1:` comments marking where implementation goes.

## What M1 fills in

- Real Postgres store implementation
- HTTP handlers that actually execute (not `StatusNotImplemented`)
- Full orchestrator flow: alert → route → agent adapter → events → persistence
- MCP Gateway basic path: HTTP/JSON in, Streamable HTTP out
- Webhook receiver connected to control plane
- Helm chart to deploy it all
- Public CI

## Development

```bash
# Run tests
make test

# Run with race detector
make test-race

# Vet
make vet

# Build
make build

# Docker image
make docker-build
```

## Configuration

See `examples/config.local.yaml` and `examples/config.corporate.yaml` for reference configs.

## Architecture

See [docs/architecture.md](docs/architecture.md) for the high-level picture.

Related repos:
- [`leloir/sdk`](https://github.com/leloir/sdk) — AgentAdapter SDK (Go module adapters import)
- [`leloir/leloir`](https://github.com/leloir/leloir) — umbrella repo with docs, CRDs, Helm chart

## License

Apache 2.0
