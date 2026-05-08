# Leloir AgentAdapter SDK

The Go SDK for building agent adapters that integrate with the Leloir incident analysis platform.

> **What is Leloir?** An open-source agentic incident analysis platform on Kubernetes. Named after Luis Federico Leloir, Argentine Nobel Laureate in Chemistry (1970), who unraveled how the body metabolizes sugars — a fitting metaphor for figuring out the root cause of complex incidents.

## What is an "AgentAdapter"?

An adapter is a thin Go translation layer (~150-300 lines) between the Leloir control plane and any AI agent. Implementing the `AgentAdapter` interface makes any agent — open source or proprietary, Go or Python or Rust (via sidecar) — a first-class citizen of Leloir.

## Quick start (5 minutes)

```bash
# 1. Bootstrap your adapter project
mkdir my-agent-adapter && cd my-agent-adapter
go mod init github.com/myorg/my-agent-adapter
go get github.com/leloir/sdk@v1.0.0

# 2. Implement the interface (5 methods)
# See examples/minimal/adapter.go for the smallest working example

# 3. Run conformance tests
go test ./...

# 4. Build container, register with Leloir, done
```

## What's in this module

```
sdk/
├── adapter/                    # The core interface and types
│   ├── interface.go           # The AgentAdapter interface (5 methods)
│   ├── types.go               # Identity, Config, Request, Event, Payloads
│   ├── errors.go              # Error types and codes
│   └── doc.go                 # Package documentation
│
├── conformance/               # Test suite for adapters
│   ├── suite.go               # Entry point (RunSuite)
│   ├── tests.go               # Individual conformance tests
│   ├── mocks.go               # Mock LLM, mock tools
│   └── options.go             # Configuration for the suite
│
└── examples/
    ├── minimal/               # ~100-line working adapter (great learning starting point)
    │   ├── adapter.go
    │   └── adapter_test.go
    │
    └── holmesgpt/             # Real reference adapter for HolmesGPT
        ├── adapter.go
        ├── client.go
        ├── events.go
        └── README.md
```

## The 5 methods you implement

```go
type AgentAdapter interface {
    Identity() AgentIdentity
    Configure(ctx context.Context, config Config) error
    HealthCheck(ctx context.Context) error
    Investigate(ctx context.Context, req InvestigateRequest) (<-chan Event, error)
    Shutdown(ctx context.Context) error
}
```

That's the entire contract. See `adapter/interface.go` for full documentation.

## Versioning

This SDK follows SemVer. The current version is `v1.0.0`.

Compatibility:
- Leloir Platform v1.x supports SDK v1.x (current) and v0.x (best effort)
- Any breaking change to the `AgentAdapter` interface = major version bump
- New event types, new optional fields = minor version bump

## License

Apache 2.0 — see [LICENSE](./LICENSE).

## Contributing

This SDK is part of the Leloir project. See the main repo at `github.com/leloir/leloir` for contribution guidelines.

---

*Built as part of Milestone 3 of the Leloir delivery plan. See [the spec](https://github.com/leloir/leloir/blob/main/docs/agentadapter-sdk-spec-v1.md) for the full design rationale.*
