<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Keleustes API Server — Design Notes

> **Status:** Scaffold-stage. This document describes the read-mostly REST
> gateway under `internal/api` (binary: `cmd/apiserver`) that backs the UI and
> `keleustesctl`. It records the seams that are in place now and the ones that
> are deliberately stubbed until later MVPs. When this disagrees with an ADR,
> the ADR wins — start from [`docs/DECISIONS.md`](../DECISIONS.md).

---

## 1. What it is

The API server is the single REST surface that both the web UI and
`keleustesctl` consume — **equal citizens over the same contract** (PROPOSAL
§17/§18). It holds no privileged business logic: it is a *view + bounded
actions* layer that translates HTTP into reads of control-plane state and, for
the three write actions, into intent that the engines act on. The UI must never
be a single point of operational failure, so anything the UI can ask for, the
CLI can ask for the same way.

At scaffold stage it serves **reads** from fixtures and answers **writes** with
an honest `501` (§6).

## 2. Contract-first codegen

`openapi/keleustes.v1.yaml` is the source of truth, not the Go types. The
server handlers, request/response models, and a typed client are generated from
it with [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) into
`internal/api/openapi/keleustes.gen.go` (the generated file is exempt from the
SPDX header and is never hand-edited).

- `task gen:api` regenerates the package from the contract.
- `task gen:check` fails CI if the checked-in `keleustes.gen.go` is stale
  relative to the contract.
- This runs in **lockstep with the UI's `ui:gen`**: the same `keleustes.v1.yaml`
  drives the TypeScript client in `ui/`. A contract change is one edit that
  regenerates both sides, so the UI and server cannot silently drift. The
  contract is the constant; both languages are derived state.

The generated `StrictServerInterface` enumerates every operation as a
`<Op>RequestObject → <Op>ResponseObject` method, so adding or removing an
endpoint is a compile error until the handler is updated.

## 3. The read-model seam (`readmodel.ReadModel`)

All reads go through one interface — `readmodel.ReadModel`
(`internal/api/readmodel/port.go`). Handlers depend only on this port; they
never reach for a Kubernetes client or a bus connection directly. This keeps
the HTTP layer ignorant of *where* state lives and lets the data source evolve
under it without touching the contract or the handlers.

The port exposes the queries the UI screens need (fleet/app listings, the
matrix, releases, promotions, targets, health, drift, environments, diff,
audit) plus the `ErrNotFound` / `ErrForbidden` sentinels the HTTP layer maps to
status codes.

Adapters, staged to match where state actually lives:

| Adapter | Status | Backs reads from | ADR |
|---|---|---|---|
| **fixtures** | **now** (default) | In-memory sample data — lets the UI and CLI run with no cluster. | — |
| **crdsource** | **now / pre-scale** | The control-plane CRDs via the controller-runtime client (etcd). Correct, but live data is scaffold-sparse and live aggregation does not hold at fleet scale. | [ADR 0005 §10](../adr/0005-distributed-runtime.md) |
| **NATS-KV + DuckDB** | **later** | Hot snapshots from NATS KV (recent window) and DuckDB-on-parquet for the matrix at 10k+ Applications, with an "as of" freshness contract. | [ADR 0005 §13](../adr/0005-distributed-runtime.md) |

The `crdsource` adapter is appropriate up to roughly the single-leader ceiling;
above it the matrix must come from the pre-computed snapshot tier, because live
fan-out queries over all Applications/Promotions are exactly the hot loops
ADR 0005 §10 forbids. Selecting the adapter is a startup flag
(`--read-model=fixtures`), not a contract concern.

## 4. The `cmd/apiserver` component

The server is a **separate component** from the controller manager
(`cmd/manager`). At MVP 0/1 it is a single binary with separate listeners for
the API and the webhook receivers in one process; the webhook receivers split
into their own `Deployment` when public exposure goes live at MVP 2
([ADR 0005 §9](../adr/0005-distributed-runtime.md)). Because the read path is
stateless, it scales horizontally on more pods, never bigger pods
([ADR 0005 §10](../adr/0005-distributed-runtime.md)).

Exposure is via Gateway API v1 — the internal/API tier, OIDC-authenticated for
humans and workload-identity/mTLS for CI
([ADR 0005 §7](../adr/0005-distributed-runtime.md)). Locally it serves plain on
`:8443` (`task run-api`); in-cluster it sits behind the IAP-fronted gateway.

## 5. Auth seam (stubbed)

Identity is OIDC and authorization is **verb-scoped and server-enforced** — the
server decides what a caller may do; the UI only renders actions
accordingly, never enforces ([ADR 0004](../adr/0004-crd-based-rbac.md)). The
policy evaluator is an in-process pure function of the RBAC CRDs
([ADR 0004 §11](../adr/0004-crd-based-rbac.md)).

That evaluator is not wired yet. The server ships an **authorizer seam** with
an **AllowAll** default so the scaffold is usable without an IdP. The seam is
deliberate: the real evaluator drops in behind the same interface (SKA-330)
without changing handlers or the contract. Do not treat AllowAll as a security
boundary — it is a placeholder for one.

## 6. The write path is an honest 501

The contract carries the **read + exactly three write actions** the UI is
allowed (Approve, Promote, Break-glass — ADR 0003 §6), surfaced as the
promotion `POST` operations (create / approve / cancel / retry). At scaffold
stage every one of them returns **`501 Not Implemented`** with a typed `Error`
body.

This is faithful to the Git-source-of-truth invariant
([ADR 0003](../adr/0003-git-source-of-truth-invariant.md)): a write must produce
a Git commit (or a CRD change that is itself in Git), and the Git-mutation
engine that does so is pending (MVP 2). A `501` is the correct answer until that
engine exists — far better than a write path that mutates cluster state directly
and quietly breaks the invariant. The handlers and contract for these
operations are in place so the UI can render the actions (disabled / with the
expected error) before the engine lands.

> **Follow-up — contract error responses.** The contract currently declares
> only the success (and `403`) responses per operation; the `not_implemented`
> code was added to the `Error` schema, but the `501` — and the cross-cutting
> `400`/`401`/`500` the server can emit — are not yet declared on each
> operation. The server returns them correctly today and the read-path
> conformance test validates the declared responses; declaring a shared error
> response set so codegen produces typed handling on both sides is a tracked
> tidy-up, not a behavior change.

---

## References

- [`docs/DECISIONS.md`](../DECISIONS.md) — the living index; cite it first.
- [ADR 0003](../adr/0003-git-source-of-truth-invariant.md) — Git source-of-truth
  invariant (the 501 write path).
- [ADR 0004](../adr/0004-crd-based-rbac.md) — CRD-based RBAC (the auth seam,
  §11 evaluator).
- [ADR 0005](../adr/0005-distributed-runtime.md) — distributed runtime
  (component shape §9, scaling §10, read tiers §13, no RDBMS).
- [`docs/design/ui-design-spec.md`](./ui-design-spec.md) — the consumer of this
  contract.
- [`openapi/keleustes.v1.yaml`](../../openapi/keleustes.v1.yaml) — the contract
  both sides are generated from.
