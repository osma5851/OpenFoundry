# MIGRATION-LOOP-STATUS — Rust → Go autonomous run

**Branch:** `frontend/settings-mfa-apikeys-sso`
**Working dir:** `/Users/torrefacto/Documents/Repositorios/OpenFoundry`
**Started:** 2026-05-06
**Mode:** /loop dynamic (self-paced)
**Push policy:** never push, never merge — local commits only

This file is the source of truth between iterations. Every iteration
reads it first, advances ONE coherent slice, runs `go build` + `go vet`
+ `go test` workspace-wide, commits, updates this file, schedules the
next wakeup.

---

## Discovery (iteration 1, 2026-05-06)

The original audit underestimated done-state. After cross-checking
files vs. NIGHTLY-SUMMARY claims:

- `libs/ml-kernel-go/domain/interop` — **already ported** (844 LOC + 327 LOC tests). `domain/interop/interop.go` mirrors `libs/ml-kernel/src/domain/interop.rs` 1:1, tests green. **Committed** as `2541be78`.
- `libs/ml-kernel-go/domain/training/{runner,execute,hyperparameter}` — **already ported** (~828 LOC + tests). `CreateTrainingJob` handler is fully wired (no longer a 501 stub). **Committed** as `069c3e9a`.
- `libs/ai-kernel-go/domain/llm/runtime.go` — **freshly ported** in this run (644 LOC + 342 LOC tests). Uncommitted at iter-1 close.

Stubs that were claimed pending but are now real production code:
- `handlers/training.CreateTrainingJob` ✅ live
- `handlers/models.CreateModelVersion` ✅ live (chains interop)

---

## True remaining work

### P1 — Unblock 8 AI/ML services (ROI máximo)

| # | Task | Status | Notes |
|---|------|--------|-------|
| 1.1 | port `libs/ml-kernel-go/domain/interop` | ✅ done | committed `2541be78` |
| 1.2 | port `libs/ai-kernel-go/domain/llm/runtime` | 🟡 ported, awaiting commit | runtime.go 644 LOC + runtime_test.go 342 LOC, build+test green |
| 1.2b | wire `handlers/chat.{CreateChatCompletion,AskCopilot,BenchmarkProviders}` | ⏳ pending | 3× 501 stubs in `libs/ai-kernel-go/handlers/chat.go:437,452,467` |
| 1.3 | port `libs/ai-kernel-go/domain/agents/executor` | ⏳ pending | Rust source 1307 LOC at `libs/ai-kernel/src/domain/agents/executor.rs` |
| 1.3b | wire `handlers/agents.ExecuteAgent` | ⏳ pending | 1× 501 stub at `libs/ai-kernel-go/handlers/agents.go:229` |
| 1.4 | port `libs/ml-kernel-go/training/{runner,execute_training}` | ✅ done | committed `069c3e9a` |
| 1.5 | wire `handlers/experiments.{ListRuns,CreateRun,UpdateRun,CompareRuns}` | ⏳ pending | 4× 501 stubs at `libs/ml-kernel-go/handlers/experiments.go:303-333`. Needs ml_runs SQL scaffolding + interop.MergeRunParams + interop.MergeRunArtifacts + evaluation.QualityScore for compare |
| 1.6 | wire `handlers/experiments.GetExperimentAssetLineage` | ⏳ pending | 1× 501 stub at `libs/ml-kernel-go/handlers/experiments.go:293`. ~459 LOC pure logic — 6-tier graph builder (experiment → runs → training jobs → model versions → models → deployments) |

### P2 — Phase 4 (Data & Ontology)

| # | Task | Status | Notes |
|---|------|--------|-------|
| 2.5 | complete `libs/cassandra-kernel` with gocql | ⏳ pending | already 233 LOC + 4 files; ~50-100 LOC remaining |
| 2.6a | port `libs/scheduling-cron` | ⏳ pending | currently 0 files |
| 2.6b | port `libs/state-machine` | ⏳ pending | currently 0 files |
| 2.6c | port `libs/saga` | ⏳ pending | currently 0 files |
| 2.6d | port `libs/search-abstraction` | ⏳ pending | currently 0 files |

### P3 — Identity / Authz follow-ups

| # | Task | Status | Notes |
|---|------|--------|-------|
| 3.7a | identity-federation slice 5b (SAML 2.0 + XML signing) | ⏳ pending | crewjam/saml + russellhaering/goxmldsig; needs IdP test certs + metadata fixtures |
| 3.7b | identity-federation slice 8 (Cedar + JWKS + Vault + SCIM) | ⏳ pending | cedar-go Option A chosen 2026-05-06; port `libs/authz-cedar-go` first as de-risking step |

### P4 — Phase 5 decision (HUMAN INPUT REQUIRED)

| # | Task | Status | Notes |
|---|------|--------|-------|
| 4.8 | go/no-go on pyo3 sidecars | ⏸ blocked-on-human | services: notebook-runtime, pipeline-build, ontology-actions. Loop must NOT decide unilaterally. |

### P5 — Hygiene

| # | Task | Status | Notes |
|---|------|--------|-------|
| 5.9 | CI job runs `buf generate` and fails on dirty tree | ⏳ pending | guards proto drift since `openfoundry-go/proto/` is empty (consumes Rust proto/ via buf) |
| 5.10 | refresh `openfoundry-go/README.md` and `INVENTORY-PHASE6.md` | ⏳ pending | both lag the actual state; README still describes Phase 0 |
| 5.11 | decide on the 16 empty lib dirs | ⏸ blocked-on-human | options: delete, or add doc.go with TODO. Sub-decision per lib. |

---

## Iteration log

### Iter 1 — 2026-05-06 (this run)

- Audited the 16 empty libs and 9 real 501 stubs.
- Confirmed P1.1, P1.4 already done (commits `2541be78`, `069c3e9a`).
- Found a fresh full port of `libs/ai-kernel-go/domain/llm/runtime.go` (644 LOC + 342 LOC tests) on disk, uncommitted.
- Verified build + vet + race tests green workspace-wide.
- Created this file.

**Next action (iter 2):** commit the runtime port, then start wiring `handlers/chat.CreateChatCompletion`.

---

## Wire-compat invariants pinned in this loop run

(filled per iteration — empty for now since no new commits yet this run)

---

## Decisions deferred for human review

1. **Phase 5 pyo3 sidecars** — go/no-go decision still required.
2. **16 empty lib dirs** — delete or stub with doc.go? Per-lib decision.
3. **Audit-sink + ai-sink Iceberg writer** (existing deferral from Run 2) — wait for iceberg-go ≥1.0.

---

## Build invariant

After every commit, this command must succeed in `openfoundry-go/`:

```
go build ./... && go vet ./... && go test -race -count=1 ./...
```

If a commit breaks this, the next iteration must revert it before
proceeding.
