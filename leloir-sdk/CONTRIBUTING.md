# Contributing to the Leloir SDK

Thanks for considering a contribution. The SDK is a small, deliberate API surface — most contributions will fall into one of these categories:

## Categories of contribution

**1. Bug fixes** — always welcome. Open an issue first if it's a behavior change.

**2. Reference adapters** — if you've built a high-quality adapter for an open-source agent (GitHub Copilot Agents, Cline, Aider, etc.), consider submitting it under `examples/`.

**3. Documentation improvements** — typo fixes, clarifications, better examples.

**4. New conformance tests** — if you can demonstrate a class of bug that the current suite doesn't catch, that's a great PR.

## Categories that need design discussion first

These changes are bigger and require an issue / RFC before code:

- **Adding a method to `AgentAdapter`** — major version bump, design discussion required
- **Adding a new event type** — minor version bump, but coordinate with Leloir core team
- **Changing the lifecycle contract** — e.g., when methods can be called concurrently
- **New helper utilities in the `adapter` package** — keep the surface small

## Development setup

```bash
git clone https://github.com/leloir/sdk
cd sdk
go mod download
make test
```

## Coding conventions

- `gofmt -w .` before committing
- `go vet ./...` must pass
- New public APIs need godoc comments
- Tests for new behavior
- Keep dependencies minimal — the SDK should have very few external deps

## Pull request checklist

- [ ] Tests pass: `make test`
- [ ] Conformance suite still passes for example adapters
- [ ] Godoc updated for any public API changes
- [ ] CHANGELOG entry added (if user-visible change)
- [ ] No new external dependencies (or justified in PR description)

## Versioning

The SDK uses SemVer:

- `MAJOR` (1.x → 2.0): breaking changes to the `AgentAdapter` interface
- `MINOR` (1.0 → 1.1): new optional methods, new event types, new helpers
- `PATCH` (1.0.0 → 1.0.1): bug fixes, doc improvements, no API change

Breaking changes require:
1. Discussion in an issue / RFC
2. Deprecation period of at least 2 minor versions
3. Migration guide

## License

By contributing, you agree your contributions are licensed under Apache 2.0
(the same license as the SDK).
