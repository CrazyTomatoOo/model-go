# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

This is a **multi-context monorepo**. The entry point is `GLOSSARY-MAP.md` at the repo root.

## Before exploring, read these

- **`GLOSSARY-MAP.md`** at the repo root: it points at one `GLOSSARY.md` per domain context (platform, training, inference, compute). Read each one relevant to the topic.
- **`docs/adr/`**: read ADRs that touch the area you're about to work in. `0001-monorepo-architecture.md` is the foundational one; context-scoped ADRs are prefixed with the context name.

If a context's glossary doesn't exist yet, **proceed silently**. Don't flag its absence; don't suggest creating it upfront. The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates them lazily when terms or decisions actually get resolved.

## File structure

```
/
├── GLOSSARY-MAP.md
├── docs/
│   ├── adr/                     ← all ADRs; context-scoped ones prefixed
│   └── glossary/
│       ├── platform.md          ← created lazily, one per context
│       ├── training.md
│       ├── inference.md
│       └── compute.md
├── cmd/                         ← deployable binaries
├── internal/                    ← per-component private code
└── pkg/                         ← shared libraries
```

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in the relevant context's glossary. Don't drift to synonyms the glossary explicitly avoids, and don't leak one context's term into another context where it means something different (e.g. "job" in training vs. inference).

If the concept you need isn't in the glossary yet, that's a signal: either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently overriding:

> _Contradicts ADR-0001 (self-built controllers), but worth reopening because…_
