<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# ADR 0009 — REST API status-code and error-body contract (RFC 9457)

- **Status:** Accepted
- **Date:** 2026-06-29
- **Deciders:** Platform Architecture (Skaphos)
- **Linear:** none dedicated — formalizes the REST contract
  (`openapi/keleustes.v1.yaml`, PROPOSAL §18) shared by the UI engine (PR #26)
  and the in-flight API-server scaffold (`feat/api-server-scaffold`,
  `internal/api`); implementation tickets to follow.
- **Related:** [ADR 0003](./0003-git-source-of-truth-invariant.md) (write =
  Git mutation → `202`/`501` semantics), [ADR 0004](./0004-crd-based-rbac.md)
  (RBAC verbs → `403`), [ADR 0005](./0005-distributed-runtime.md) (eventual
  consistency → stale reads stay `200`, not `503`),
  [ADR 0008](./0008-resource-identity-model.md) (natural-key addressing →
  `/{name}` `404` semantics). Also: PROPOSAL §18 (API requirements),
  `openapi/keleustes.v1.yaml` (the contract artifact), and
  `docs/design/ui-design-spec.md` §5 (the error-class requirement this ADR
  backs).
- **Supersedes:** the ad-hoc per-endpoint status codes and the `{code,message}`
  `Error` schema currently in `openapi/keleustes.v1.yaml`. Refines PROPOSAL §18
  (adds the response contract the endpoint list omits).

## Context and Problem Statement

`openapi/keleustes.v1.yaml` is the single source of truth shared by three
consumers: the UI typed client (`openapi-typescript`), the UI mock backend (MSW
handlers + fixtures), and the future Go API server (`internal/api`). It is a REST
contract — once the UI, `keleustesctl`, and the server bake in assumptions about
what a call returns, those assumptions are expensive to unwind. Today the
response half of that contract is unspecified and already drifting:

1. **The committed contract is inconsistent and partly unreachable.** Most read
   endpoints document only `200`; only `/applications/{name}` and
   `/promotions/{id}` document `404`. Writes mix `202` (`POST /promotions`),
   `200` (`approve`, `cancel`, `retry`), and `403` (some, not all). The shared
   `Error` schema enumerates `not_found, forbidden, unauthenticated, conflict,
   degraded, invalid` — but `unauthenticated`, `conflict`, `degraded`, and
   `invalid` are attached to **no** response anywhere in `paths`. They are
   declared but unreachable.

2. **The implementation had begun to diverge from the contract — which this ADR
   resolves.** The `feat/api-server-scaffold` branch initially carried its own
   mapping in `internal/api/server/errors.go` (`errNotImplemented → 501`,
   `ErrNotFound → 404`, `ErrForbidden → 403`, request-binding faults →
   `400/invalid`, default → `500/degraded` with the cause hidden on the wire and
   logged with the request id, and `401` from the auth middleware), and had added
   a `not_implemented` code to its local OpenAPI enum that `main` lacked. That
   divergence was the forcing function for this ADR, and it is closed by the same
   change set that adopts it: the scaffold has been realigned onto this contract
   (§5, follow-up #4) — the server now emits `application/problem+json` from one
   error sink generated against this spec, so the spec, the Go server, and the
   typed clients share a single mapping rather than drifting.

3. **The UI requires distinctions the contract cannot currently back.** The UI
   design spec §5 mandates that the client distinguish *auth error* (re-auth),
   *permission denied* (you lack verb X), *not found*, and *backend degraded* —
   and render each differently (re-auth flow vs. disabled-with-tooltip vs.
   not-found vs. stale-snapshot banner). That is only possible if the server
   returns distinguishable, documented status codes plus a stable machine
   discriminator.

4. **Prior ADRs already imply specific HTTP semantics that nothing has written
   down.** ADR 0003 makes every write a Git mutation, so a successful write
   "accepts intent" and the effect lands later — that is `202`, not `200`, and an
   inert write surface (engine not yet shipped) is honestly `501`, not a faked
   `200` or a misleading `404`. ADR 0004 makes a missing verb a `403`. ADR 0005
   makes reads eventually consistent — a *stale* snapshot is not an error and
   must stay `200` (with `asOf`), distinct from an *unavailable* backend.

The forcing function: lock one status-code-and-error-body contract for both the
read and write surfaces — covering success, client-fault, auth, and server-fault
codes — before the UI client, `keleustesctl`, and the Go server bake in
incompatible assumptions.

## Decision Drivers

- One contract, three consumers (OpenAPI spec, Go server, UI/CLI clients) that
  must agree byte-for-byte.
- The UI design-spec §5 error classes need distinguishable codes plus a stable
  machine discriminator to branch on — not prose.
- Must encode the HTTP semantics that ADR 0003 (async writes), ADR 0004 (RBAC
  denials), and ADR 0005 (stale-but-available reads) already imply.
- Forward compatibility: the Source/Sync/Promotion/Git-Mutation/Policy/Health
  engines each arrive across MVP 0 → 4 and will add failure modes. Naming the
  codes up front makes adding them additive, not a contract break.
- Errors must be machine-actionable and tool-friendly, not a bespoke shape every
  client re-learns.

## Considered Options

- **A — Status quo.** Per-endpoint codes added ad hoc; keep the `{code,message}`
  body with its declared-but-unreachable enum.
- **B — Formalize `{code,message}`, minimal live-only set.** Keep the existing
  body shape (already wired through generated Go + the UI typed client); document
  only the codes MVP 0 actually emits (`200/202/400/401/403/404/500/501`); add
  `409/429/503` in a later ADR when an engine needs them.
- **C — RFC 9457 Problem Details, full canonical matrix, live-vs-reserved
  markers.** *(chosen)* Replace the bespoke body with `application/problem+json`;
  document the complete read + write status set up front, marking which codes are
  live in MVP 0 and which are reserved (named, not yet emitted).

## Decision Outcome

Chosen option: **C**. It is the only option that backs the UI §5 error classes
with a standard, extensible envelope, writes down the ADR 0003/0004/0005 HTTP
semantics, and lets future engines add failure codes without breaking clients.
We accept that it is a breaking change to the in-flight scaffold and the current
contract (see Consequences) because the contract has not shipped to any external
consumer yet — now is the cheapest moment to make it.

### 1. Error body: RFC 9457 Problem Details

Every error response uses `Content-Type: application/problem+json` with the
RFC 9457 members `type`, `title`, `status`, `detail`, and `instance`. The `type`
is a stable URI under `https://keleustes.skaphos.io/errors/<slug>` and is the
**machine discriminator** — it replaces the old `code` enum (the existing enum
values become `type` slugs: `not_found`, `forbidden`, `unauthenticated`,
`invalid`, `conflict`, `degraded`, `not_implemented`, `too_many_requests`,
`step_up_required`). Clients branch on `type`, never on `detail` prose. Problem
Details permits extension members; we use them deliberately:

- `403` carries `verb` + `resource` so the UI can render
  "requires `promote` on Application/…" (design spec §5).
- `400` validation failures carry an `errors[]` array of per-field problems.
- `503` carries `asOf` and the lagging `region` when known.
- `step_up_required` (`401`) is accompanied by a `WWW-Authenticate` challenge
  header carrying the required `acr_values`/`max_age` (RFC 9470 — see §4).

`detail` for a `degraded` (`500`) response is a fixed string; the real cause is
logged with the request id and never leaks on the wire (the scaffold already
does this — it is preserved, not changed).

### 2. Read endpoints — canonical set

`GET` is safe and side-effect-free; collection reads return `200` with a
(possibly empty) array and **never** `404`.

| Code | `type` slug | When |
|------|-------------|------|
| `200` | — | Resource or collection returned. Stale-but-available data is still `200` (see below). |
| `400` | `invalid` | Malformed path/query param (bad enum, unparseable date). |
| `401` | `unauthenticated` | Missing/expired token. UI → re-auth. |
| `403` | `forbidden` | Authenticated but lacks the read verb (ADR 0004). Body carries `verb`+`resource`. |
| `404` | `not_found` | Addressed single resource does not exist (single-GET only). |
| `500` | `degraded` | Unexpected server fault; cause hidden on wire, logged. |

**Stale snapshots stay `200`, not `503`.** The matrix and fleet rollups are
eventually consistent (ADR 0005); the response already carries an `asOf`
timestamp (and a lagging-region marker on the matrix). A stale-but-served
snapshot is *not* an error — the UI renders the "as of" / stale banner (design
spec §5) from `asOf`, not from a status code. `503` is reserved for the backend
being *unreachable*, not merely behind (see §4).

**Authorization vs. existence (`403` within, `404` across).** Within a Project
the caller can see, a missing verb is a `403` carrying `verb`+`resource` (so the
UI can disable-with-tooltip, design spec §5). Across a Project/tenant boundary
the caller has *no* visibility into (ADR 0004's delegation boundary), the API
returns `404 not_found` — it must not confirm a resource's existence to someone
who cannot see the Project. The same rule governs the write endpoints. `404` is
therefore overloaded by design: "truly absent" and "exists but outside your
tenant" are deliberately indistinguishable to the caller.

### 3. Write endpoints — canonical set and async semantics

Writes are `POST`. Under ADR 0003 a write accepts *intent*; the desired-state
change lands later as a Git commit + reconcile. The success code reflects that:

| Endpoint | Live code | Why |
|----------|-----------|-----|
| `POST /promotions` | `202` | Opens a Git PR on approval — effect is async (ADR 0003). |
| `POST /promotions/{id}/cancel` | `202` | Async unwind. |
| `POST /promotions/{id}/retry` | `202` | Re-triggers async work. |
| `POST /promotions/{id}/approve` | `200` | The decision is recorded **synchronously**; the sync it unblocks is still async, but the approval itself is complete. |

This **changes the current contract**, which returns `200` for `cancel`/`retry`:
those move to `202` so the code tells the truth about asynchrony. Write
client-fault and auth codes mirror the read set: `400 invalid` (malformed body;
field errors in `errors[]`), `401`, `403` (with `verb`+`resource`), `404`
(write against a missing resource), `500 degraded`.

**Inert write surfaces return `501 not_implemented`.** Until the Git-mutation
engine lands (ADR 0003), all four promotion writes return `501` — the endpoint is
contract-defined but its engine has not shipped. This matches the scaffold's
`errNotImplemented` sentinel and makes "exists but does nothing yet" a
first-class, documented state rather than a `404` or a misleading `200`.

**Writes accept an `Idempotency-Key`.** Every `POST` accepts a client-chosen
`Idempotency-Key` header; replaying a key returns the original response (the same
`202`/`200` and `Promotion`) without opening a second PR or recording a second
decision — satisfying the idempotent-and-bounded hard invariant against
dropped-response retries, and pairing with the existing `webhook-dedup` NATS KV
bucket. Reusing a key with a *different* body is a `409 conflict` (§4).

### 4. Reserved codes (named now, emitted later)

These are documented in the contract with a `type` slug and semantics so a server
may begin returning them **without a contract-breaking change**. None are emitted
in MVP 0:

- `401 step_up_required` — authenticated, but the action needs stronger
  assurance (the elevated/break-glass path). Returned as `401` with a
  `WWW-Authenticate` challenge carrying the required `acr_values`/`max_age`
  (OAuth Step-Up Authentication Challenge, RFC 9470), distinct from
  `401 unauthenticated` (no/expired token) by its `type` slug. Reserved until the
  break-glass endpoint lands — which the contract's `info` block already names as
  a write action but `paths` omits.
- `409 conflict` — concurrency / precondition conflict. When writes adopt
  `If-Match`/ETag (CRD `resourceVersion`), a stale precondition is a `409`; a
  reused `Idempotency-Key` (§3) with a mismatched body is also a `409`. Reserved
  until the write engines land.
- `429 too_many_requests` — API-tier rate limiting / backpressure; carries
  `Retry-After`.
- `503 degraded` — the read-model backend (NATS/snapshot) or a regional agent is
  **unreachable** (distinct from `200 + asOf` stale and from `500` internal
  fault); carries `Retry-After`/`asOf` when known.

### 5. One mapping, enforced in one place

The status↔`type` mapping is defined once and shared: the server's single error
sink (`internal/api/server/errors.go` `classify`), the OpenAPI `responses`
components, and the UI client's error-class switch (driven by the `type` slug).
Handlers raise typed/sentinel errors; they do not pick status codes inline. The
OpenAPI spec is the source the Go types and the UI client are generated from, so
the three cannot silently drift again.

### Consequences

- **Positive**
  - One agreed contract across spec, server, and clients; the divergence in §2 of
    the Context can't recur because all three are generated from / checked against
    the OpenAPI file.
  - The UI design-spec §5 error classes are backed by distinguishable codes plus
    a stable `type` discriminator.
  - `202`-for-async is honest about ADR 0003: the UI's "Promotion PR opened →
    github.com/…" toast matches a code that says "accepted, effect pending."
  - Reserved codes make future engine failure modes additive, not breaking.
  - Cross-tenant existence is hidden by the `403`-within / `404`-across rule, so
    the contract never leaks resource names across an ADR 0004 Project boundary.
  - `Idempotency-Key` lets clients safely retry a write whose `202`/`200`
    response was dropped, without double-opening a PR.
  - `application/problem+json` is an IETF standard many clients and proxies render
    natively; `errors[]` gives per-field validation detail the old shape couldn't.
- **Negative**
  - **Breaking change to in-flight work — already absorbed in this change set.**
    `openapi/keleustes.v1.yaml` moves from `{code,message}` to `Problem`, and
    `internal/api/server/errors.go` (`classify`/`writeError`/`message`), the
    generated `internal/api/openapi` types, and the UI typed client are
    regenerated against it. That rework has been applied on
    `feat/api-server-scaffold` here — the server emits `application/problem+json`,
    `cancel`/`retry` return `202`, and the `403` body carries `verb`+`resource` —
    so the branch ships in conformance instead of needing a later rebase. The MSW
    error fixtures and the UI's design-spec §5 error-class switch remain a
    UI-side follow-up (item 5).
  - `application/problem+json` is a second content type the UI must handle
    alongside `application/json` success bodies.
  - RFC 9457 `type` URIs imply we own a stable error-slug namespace (and decide
    whether the URIs resolve to docs).
  - Reserved codes risk "documented but never implemented" rot if not tracked.
- **Neutral**
  - `keleustesctl` and the UI share the one envelope.
  - `instance` duplicates the request path the client already knows.
  - Problem Details allows arbitrary extension members, so we keep an internal
    allowlist (`verb`, `resource`, `errors`, `asOf`, `region`, `Retry-After`) to
    prevent member sprawl.

## Pros and Cons of the Options

### A — Status quo
- Good: zero work.
- Bad: contract and server already diverge; declared-but-unreachable enum values;
  cannot back UI §5; lies about async writes (`200`).

### B — Formalize `{code,message}`, minimal set
- Good: lowest churn; keeps the generated Go + UI client as-is; honest about
  emitting only what exists.
- Bad: a bespoke error shape every client re-learns; no structured per-field
  validation; "minimal" means every new engine failure mode is a contract change,
  i.e. client-breaking churn deferred, not avoided.

### C — RFC 9457 Problem Details, full matrix *(chosen)*
- Good: standard, extensible envelope; full read+write matrix and async semantics
  documented up front; reserved codes make growth additive; backs UI §5.
- Bad: breaking change to the in-flight scaffold + contract; second content type;
  obliges us to own an error-slug namespace.

## Compliance and follow-ups

1. **`docs/DECISIONS.md`** — add the ADR 0009 row. *(this PR)*
2. **PROPOSAL §18** — add the `> See ADR 0009` marker noting the response
   contract lives here now. *(this PR)*
3. **`openapi/keleustes.v1.yaml`** — DONE in this change set: the `Error` schema
   is replaced by `Problem` (`application/problem+json`); the §2/§3 status matrix
   and the §4 reserved-code responses (`409`/`429`/`503`) are attached as
   documented components; the `Idempotency-Key` request header is documented on
   writes. Deferred until their flows land: the `WWW-Authenticate` challenge for
   `step_up_required` (break-glass) and runtime enforcement of the `403`-within /
   `404`-across rule. *(remainder: follow-up)*
4. **`feat/api-server-scaffold`** — DONE in this change set: `internal/api/server/errors.go`
   (`classify`/`writeError`/`message`) emits `application/problem+json`,
   `internal/api/openapi` is regenerated against the new contract, `cancel`/`retry`
   return `202`, the `501 not_implemented` write behavior is preserved, and the
   `403` body carries `verb`+`resource` (ADR 0004) while `500` hides its cause.
5. **UI typed client + MSW fixtures** — regenerate against the new contract; wire
   the design-spec §5 error-class switch on the `type` slug. *(follow-up)*
6. **Error registry** — mint stable `type` slugs under
   `https://keleustes.skaphos.io/errors/<slug>` (opaque-but-stable for now;
   making the URIs resolve to human docs later is non-breaking). *(follow-up /
   new Linear work)*
7. **Break-glass endpoint** — the `info` block names break-glass as a write
   action but `paths` has none. Add it, returning the `401 step_up_required`
   challenge (§4), when the break-glass flow is specified (RBAC plan §3). *(new
   Linear work)*
