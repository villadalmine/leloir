# Leloir — Project Index

**One-line summary:** Open-source agentic incident analysis platform for Kubernetes. Apache 2.0. Named after Luis Federico Leloir, Argentine Nobel Laureate in Chemistry.

**Where you are:** This bundle contains everything you need to start the project. Read this index first; it tells you which file to open next based on what you want to do.

---

## 🚦 Read in this order

### If you have 5 minutes
1. **`docs/STATUS.md`** — what's decided, what's pending, what's next. Skim this first.
2. **`docs/leloir-README.md`** — the public-facing project pitch. Use as basis for the first commit.

### If you have 30 minutes (recommended for someone new to the project)
1. **`docs/ONBOARDING.md`** — guided tour of the project for a new contributor.
2. **`docs/leloir-architecture.html`** — open in a browser, scroll through the visual architecture.
3. **`docs/leloir-milestone-0-plan.md`** — the 2-week PoC plan you'll execute first.

### If you have 2 hours and you want everything
Read the master design doc:
- **`docs/proposal-leloir-platform-v5.3.md`** — 2,042 lines, the full architectural design with all 12 strategic decisions resolved.

---

## 🛠 When you're ready to build

Run the bootstrap script:
```bash
./scripts/bootstrap.sh
```

It will:
1. Verify prereqs (Go 1.22+, Docker, kubectl, helm, gh CLI)
2. Check if the GitHub org `leloir` is available
3. Check if `leloir.dev` and `leloir.io` domains are available
4. Unpack the SDK and control plane skeletons into a working repo
5. Initialize git and make the first commit
6. Print the next steps

You can also run individual phases:
```bash
./scripts/bootstrap.sh --check-prereqs     # only verify tools installed
./scripts/bootstrap.sh --check-naming      # only verify github/domain availability
./scripts/bootstrap.sh --extract           # only unpack the skeletons
./scripts/bootstrap.sh --init-git          # only init git + first commit
```

---

## 📦 What's in this bundle

```
leloir-bundle/
├── INDEX.md                                 ← you are here
├── docs/
│   ├── STATUS.md                            What's decided / pending (5-min skim)
│   ├── ONBOARDING.md                        Guided tour for a new contributor (30 min)
│   ├── leloir-README.md                     Public README (basis for first commit)
│   ├── leloir-milestone-0-plan.md           The 2-week PoC plan, day by day
│   ├── leloir-architecture.html             Visual architecture (open in browser)
│   ├── proposal-leloir-platform-v5.3.md     Full design doc (master reference)
│   └── leloir-agentadapter-sdk-spec-v1.md   SDK spec (informs M3)
├── assets/
│   ├── leloir-sdk-v1-skeleton.zip           Go SDK code (~2,370 lines)
│   ├── leloir-controlplane-skeleton.zip     Go control plane code (~2,100 lines)
│   └── leloir-crds-v1.zip                   K8s CRDs + Helm value profiles
└── scripts/
    └── bootstrap.sh                         Automated Day-1 setup
```

---

## 📚 Reference: What each artifact answers

| Question | File |
|---|---|
| What is this project? | `docs/leloir-README.md` |
| Why these decisions? | `docs/proposal-leloir-platform-v5.3.md` |
| What do I do first? | `docs/leloir-milestone-0-plan.md` |
| What's the API contract? | `docs/leloir-agentadapter-sdk-spec-v1.md` |
| What does the architecture look like? | `docs/leloir-architecture.html` |
| How do I start coding? | `assets/*.zip` + `scripts/bootstrap.sh` |
| What's pending to decide? | `docs/STATUS.md` |
| I'm new, where do I start? | `docs/ONBOARDING.md` |

---

## 🎯 Project goal recap

Build an **agnostic, open-source platform** where any AI agent (HolmesGPT, Hermes, OpenCode, Claude Code, custom) can investigate incidents on Kubernetes, including invoking other agents (A2A — agent-to-agent), with unified cost tracking, audit, and approvals. Two deployment profiles: corporate (Azure OpenAI, Vault, strict audit) and local (Anthropic, Dex, relaxed).

**Timeline:** ~25-30 weeks to v1.0.
**Team:** 1 Go backend + 1 React frontend + 1 platform/SRE + part-time PM/lead.
**License:** Apache 2.0.

---

*If you got this far, you're ready. Start with `docs/STATUS.md` to see where things stand, then `scripts/bootstrap.sh` when you're ready to build.*
