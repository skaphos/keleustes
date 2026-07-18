<!--
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
-->

# Agent Transport Interface

- **Status:** Draft — 2026-05-17
- **Linear:** SKA-321. Blocks SKA-363 (MVP 2 Agent v1 — NATS leaf, gitops-engine, claims SyncRuns) and SKA-378 (MVP 3 optional gRPC agent transport).
- **Promotes into:** a future ADR co-located with ADR 0005. Until then, authoritative for any code that touches hub↔agent communication.
- **Resolves:** [`docs/plans/2026-05-distributed-runtime-architecture.md`](./2026-05-distributed-runtime-architecture.md) §13 Q8 (transport pluggability timing — interface from day one) and §13 Q9 (`Agent` CR — yes, with the shape below). Also concretizes ADR 0005 §7.3 (agent identity) and §11.5 (DeploymentTarget ownership claim).
- **Related:** [ADR 0005](../adr/0005-distributed-runtime.md) §5 (NATS leaf default), §7.3 (NKey + JWT), §11.5 (per-DeploymentTarget ownership), §168 (Agent is a CR); [SKA-322 Audit Schema](./2026-05-audit-event-schema.md) §6.3 (`actor.type=agent`) + §13.8 (agent verbs); [SKA-323 RBAC CRD Shapes](./2026-05-rbac-crd-shapes.md) §3 (`IdentityProvider.kind=NATSNKey`) + §12 (Agent CR not in RBAC alphabet); [SKA-324 JetStream Layout](./2026-05-jetstream-subject-and-stream-layout.md) §3.2 (`keleustes.agent.>` class), §5.1 (`keleustes-agent` stream — WorkQueue, 1h retention), §6 (`agent-presence` + `controller-locks` KV buckets); [SKA-328 Sharded Controllers](./2026-05-sharded-controller-pattern.md) §5 (`controller-locks` claim protocol the agent reuses).

## 1. Purpose and Scope

ADR 0005 named NATS leaf nodes as the default agent↔hub transport
and committed to pluggability for gRPC / Tailscale / Teleport /
Cloudflare Tunnel "from day one or later." This plan picks "day
one" — an interface that admits NATS today and other transports
without changing callers — and pins the concrete NATS leaf
implementation that ships with MVP 2.

**In scope:**

- The `internal/agent/transport.Transport` Go interface (verbs,
  types, lifecycle).
- The NATS leaf reference implementation (outbound-only
  connection model, NKey + JWT auth, JetStream consumer group
  for work claim, WebSocket-on-443 fallback).
- The `Agent` CR shape (deferred by ADR 0005 §168, now spec'd).
- The work-claim protocol: deterministic 1:N agent:target ownership
  via the existing `controller-locks` NATS KV bucket.
- Heartbeat shape and cadence.
- Large-payload transport via NATS Object Store.
- Pre-registered agent registration flow (`keleustesctl agent
  register`) + the `Agent` CR lifecycle that supports it.
- Identity propagation: how a SyncRun's originating actor reaches
  the agent that ultimately applies it.
- Reconnection + replay semantics on partition recovery.
- Backpressure: what happens when the agent can't keep up.
- Failure modes (disconnect, dropped credentials, expired JWT,
  rate-limit).
- Sketches of alternate-transport implementations (gRPC bidi,
  HTTP/2 long-poll) so the interface is provably general.

**Out of scope:**

- The agent's *internal* architecture (sync engine wrapping,
  gitops-engine pkg/sync usage, cluster cache management) — that
  is SKA-363's MVP 2 implementation work.
- The NATS supercluster topology (one logical leaf-mesh per ADR
  0005 §5; geographic placement is operator concern).
- IAP / Cloudflare Access / oauth2-proxy integration recipes
  (MVP 3 work; runtime plan §13 Q12).
- The Optional gRPC Transport (SKA-378, MVP 3) — sketched here as
  a generality test, implemented there.

## 2. Decisions (Short Form)

The four decisions taken explicitly before drafting:

1. **Rich typed `Transport` interface; NATS leaf is the only
   real implementation through MVP 2.** A fake implementation
   ships in `transport/faketransport` for tests and proves the
   interface is general. SKA-378's gRPC implementation lands in
   MVP 3 without changing callers. Resolves runtime plan §13 Q8.
2. **Deterministic 1:N agent:target ownership.** An agent claims
   `DeploymentTarget`s via the same `controller-locks` NATS KV
   bucket (SKA-324 §6 / SKA-328 §5) — 30 s lease,
   heartbeat-extended. Per-target cluster cache stays hot.
   Failover: lease expires, another agent claims, accepts the
   re-warm cost.
3. **NATS Object Store for transient agent payloads; existing
   object storage stays for content-addressed cache.** Two buckets,
   two concerns. The agent's credential surface is NATS-only; ADR
   0005 §7.3's "outbound-only" promise holds.
4. **Pre-registered agents.** `keleustesctl agent register
   --target=<dt-name>` generates the NKey, creates the `Agent`
   CR, prints the seed for the agent's env. Agent's first connect
   validates against an existing record. No auto-approve;
   `agent-registered` audit event fires on the admin's action.

## 3. Where This Fits

```
                    ┌─────────────────────────────────────┐
                    │            HUB (region 1)           │
                    │                                     │
                    │   ┌──────────┐    ┌──────────┐      │
                    │   │ Sync     │    │ Promotion│      │
                    │   │ Engine   │    │ Engine   │      │
                    │   └────┬─────┘    └────┬─────┘      │
                    │        │               │            │
                    │        ▼               ▼            │
                    │   ┌─────────────────────────┐       │
                    │   │ internal/agent (hub side)│      │
                    │   │  - Transport facade      │      │
                    │   │  - Claim coordinator     │      │
                    │   └────┬────────────────────┘       │
                    │        │                            │
                    └────────┼────────────────────────────┘
                             │
                             │ NATS supercluster
                             │ (leaf-mesh; outbound from agents)
                             │
       ┌─────────────────────┴─────────────────────┐
       │                                           │
       ▼                                           ▼
  ┌──────────────┐                            ┌──────────────┐
  │ AGENT pod    │                            │ AGENT pod    │
  │ (region 2,   │                            │ (region 3,   │
  │  prod-eu)    │                            │  prod-asia)  │
  │              │                            │              │
  │ Transport ◀──── Transport interface ────▶ Transport     │
  │   │          │                            │   │          │
  │   ▼          │                            │   ▼          │
  │ Sync wrapper │                            │ Sync wrapper │
  │ pkg/sync ────┼────► target K8s cluster    │ pkg/sync ────┼────► target K8s cluster
  │ pkg/cache    │                            │ pkg/cache    │
  └──────────────┘                            └──────────────┘
```

The same `internal/agent/transport.Transport` interface is
implemented on both the hub and the agent — the hub uses it to
publish work and receive results; the agent uses it to claim
work and publish results. One interface, two callers, one wire
protocol.

The hub never connects *to* the agent. Every connection is
outbound from the agent — this is the architectural promise ADR
0005 §7.3 made (the security implication: agents traverse only
egress firewalls and proxies, never ingress). The NATS leaf
protocol is request/reply-capable over the same outbound TCP, so
the hub-initiated paths (sending work to an agent) ride RPC-style
replies on the agent's subscription.

## 4. The Transport Interface

```go
// internal/agent/transport/transport.go
package transport

import (
    "context"
    "io"
    "time"
)

// Transport is the agent-to-hub communication contract. Both the
// hub side (transport/leaf/leafserver.go) and the agent side
// (transport/leaf/leafclient.go) implement this interface against
// the same NATS leaf-node mesh; gRPC and HTTP/2 long-poll
// implementations land as alternates per ADR 0005 §5.
//
// Lifecycle: Connect → operate → Disconnect. A Transport in
// "operating" state is connected to a quorum of NATS leaf nodes
// (or the alternate transport's equivalent), has a valid JWT,
// and has at least one active subscription.
type Transport interface {
    // Connect dials the hub and authenticates. Returns when the
    // first successful handshake completes. Subsequent
    // disconnections are recovered transparently per §10.
    //
    // The context governs only the initial dial; the connection
    // outlives ctx.
    Connect(ctx context.Context, cfg ConnectConfig) error

    // Disconnect releases all claims, flushes pending publishes,
    // and tears down the connection. Idempotent.
    Disconnect(ctx context.Context) error

    // ClaimWork attempts to claim a unit of work — typically a
    // DeploymentTarget assignment. Returns the claim handle on
    // success; the handle's lease must be heartbeat-extended via
    // Heartbeat() to remain valid.
    //
    // Implementations: NATS leaf uses the controller-locks KV
    // bucket (SKA-324 §6). gRPC uses a server-side ClaimWork RPC.
    ClaimWork(ctx context.Context, key string) (Claim, error)

    // ReleaseClaim relinquishes a previously-claimed unit of work
    // before the lease would otherwise expire. Called on graceful
    // agent shutdown.
    ReleaseClaim(ctx context.Context, claim Claim) error

    // Heartbeat extends every active claim's lease. The agent
    // calls this on a fixed period (default 10s, lease TTL 30s).
    // Returns the set of claims that could not be extended (lease
    // lost during a partition); the caller stops processing those.
    Heartbeat(ctx context.Context) (lost []Claim, err error)

    // PublishEvent sends an event up the bus. Subject construction
    // goes through internal/events/subject.For() per SKA-324 §11.1.
    // PublishEvent is synchronous through the broker ack.
    PublishEvent(ctx context.Context, env EventEnvelope) error

    // StreamLargePayload uploads a large blob (cluster cache
    // snapshot, multi-MiB render output, diff result) to NATS
    // Object Store and returns the pointer the hub or another
    // agent can fetch from. Caller closes the reader.
    StreamLargePayload(ctx context.Context, name string, r io.Reader) (LargePayloadRef, error)

    // FetchLargePayload is the read side of StreamLargePayload.
    FetchLargePayload(ctx context.Context, ref LargePayloadRef) (io.ReadCloser, error)

    // Subscribe registers a handler for incoming messages on a
    // subject pattern. Used by the agent for hub→agent commands
    // (sync request, cancel, drain) and by the hub for
    // agent→hub events (Deployment, HealthCheck reports).
    //
    // Handlers must complete within ctx; if they return an error
    // the message is nacked and redelivered per the stream's
    // retry policy.
    Subscribe(ctx context.Context, subject string, handler Handler) (Subscription, error)

    // Status returns a snapshot of the transport's current state
    // for liveness probes and the keleustesctl agent status
    // command.
    Status() Status
}

// ConnectConfig carries the credentials and endpoints the
// implementation needs.
type ConnectConfig struct {
    // AgentName is the metadata.name of the Agent CR (also the
    // actor.subject in SKA-322 §6.3 envelopes).
    AgentName string

    // NKeySeed is the agent's NKey private seed (loaded from a
    // Kubernetes Secret or local file). Used to sign the JWT
    // handshake.
    NKeySeed []byte

    // JWT is the signed JWT issued at registration time, scoping
    // the agent to specific DeploymentTargets and subject
    // patterns. See §5.2.
    JWT string

    // ServerURLs are the NATS leaf-node endpoints (e.g.,
    // ["nats://hub.us-east.skaphos.io:4222"]). When empty,
    // implementations fall back to the in-cluster service
    // (hub-internal agents only).
    ServerURLs []string

    // PreferWebSocket forces the WebSocket-on-443 fallback (§5.4)
    // even when raw TCP would succeed. For environments behind
    // restrictive HTTPS proxies.
    PreferWebSocket bool

    // TLSConfig is the TLS configuration. Empty disables TLS
    // (dev/test only); production deployments always set it.
    TLSConfig *tls.Config
}

// Claim is an opaque handle to a successfully-claimed work unit.
// Implementations carry whatever they need to extend / release
// the lease.
type Claim interface {
    Key() string       // the claimed key (e.g., "deploymenttarget/prod-us-east-1")
    ExpiresAt() time.Time
}

// EventEnvelope is the wire-format envelope the agent publishes
// upward. Maps 1:1 onto the SKA-322 audit envelope shape; the
// transport does not interpret the contents.
type EventEnvelope struct {
    Subject  string          // built by internal/events/subject.For()
    Payload  json.RawMessage // canonical JSON per SKA-322 §4.1
    MsgID    string          // ULID; ack-dedup via Nats-Msg-Id header
}

// LargePayloadRef is the pointer returned by StreamLargePayload
// and consumed by FetchLargePayload. Contains the NATS Object
// Store bucket + key + content hash for verification.
type LargePayloadRef struct {
    Bucket string `json:"bucket"`
    Key    string `json:"key"`
    Hash   string `json:"hash"` // sha256:...
    Size   int64  `json:"size"`
}

// Handler processes one incoming message.
type Handler func(ctx context.Context, msg Message) error

// Message is the inbound counterpart to EventEnvelope.
type Message struct {
    Subject string
    Data    []byte
    Headers map[string]string
    Ack     func() error
    Nack    func(reason string) error
}

// Subscription is the lifecycle handle for an active Subscribe.
type Subscription interface {
    Unsubscribe() error
    Pending() int
}

// Status reports current connection health for probes.
type Status struct {
    Connected         bool
    Endpoint          string
    ActiveClaims      int
    PendingPublishes  int
    LastHeartbeat     time.Time
    AuthMethod        string // "nkey+jwt" or "tls-mtls"
    LeafProtocol      string // "tcp" or "ws"
}
```

Eight verbs total: `Connect`, `Disconnect`, `ClaimWork`,
`ReleaseClaim`, `Heartbeat`, `PublishEvent`, `StreamLargePayload`,
`FetchLargePayload`, plus `Subscribe` and `Status`. The two
read-side helpers (`FetchLargePayload`, `Subscribe`) bring the
count to 10 — the ticket's list (`ClaimWork`, `PublishEvent`,
`StreamLargePayload`, `Heartbeat`, `Subscribe`, `Disconnect`)
expanded to make the lifecycle + claim-release symmetry explicit.

## 5. NATS Leaf Reference Implementation

### 5.1 Outbound-only connection model

Agents dial outbound to a NATS leaf-node endpoint that the hub
exposes (typically a `Service` of type `LoadBalancer` with a
public address, or `TLSRoute` per runtime plan §13 Q12). The leaf
node bridges the agent into the hub's NATS supercluster; the agent
itself is *just* a NATS client from the broker's perspective.

Concretely:

- The agent's pod has no `Service` of its own — nothing connects
  *to* the agent.
- Every NATS subject the agent uses (claim KV, work queue,
  publish/subscribe) rides the single outbound TCP connection.
- Request/reply RPCs (hub asking the agent to do something) use
  NATS request semantics over the agent's pre-established
  subscription — the agent listens, the hub publishes to a
  subject only that agent subscribes to, the agent replies on a
  generated inbox subject.

This satisfies ADR 0005 §7.3 (`outbound-only`) and §11.2's "no
cluster-cache aggregation on the hub" (the agent's cache stays
local; the hub asks for projections via RPC, never streams the
whole cache).

### 5.2 NKey + JWT auth

Two layers, both required:

1. **NKey signing.** The agent presents its NKey public key on
   connect; the hub challenges with a nonce; the agent signs the
   nonce with the NKey private seed. Proves possession of the
   NKey without ever transmitting the seed. Standard NATS NKey
   flow.
2. **Signed JWT scoping.** A signed JWT (issued by the hub at
   registration time per §7) carries the agent's permissions:
   which `DeploymentTarget`s the agent is allowed to claim, which
   NATS subjects it may publish to, which subjects it may
   subscribe to. The JWT's claims map directly to the NATS
   account/user permission model.

```json
// Example agent JWT claims
{
  "sub":  "agent-prod-us-east-7",         // matches Agent CR metadata.name
  "iss":  "hub-us-east-1",                // hub identity
  "iat":  1747497931,
  "exp":  1779033931,                     // 1 year default; rotatable
  "nats": {
    "pub": {
      "allow": [
        "keleustes.agent.cluster.agent.prod-us-east-7.*",
        "keleustes.events.>.deployment.>.status",
        "keleustes.events.>.healthcheck.>.report",
        "keleustes.events.>.syncrun.>.applied",
        "keleustes.events.>.syncrun.>.failed",
        "keleustes.audit.>"
      ]
    },
    "sub": {
      "allow": [
        "keleustes.agent.cluster.agent.prod-us-east-7.command.>",
        "keleustes.agent.cluster.deploymenttarget.>.claim-response"
      ]
    },
    "data": -1,                           // unlimited (rely on stream max_bytes)
    "payload": 8388608,                   // 8 MiB inline cap; large via Object Store
    "subs": 256                           // max concurrent subscriptions
  },
  "keleustes": {
    "deploymentTargets": ["prod-us-east-1", "prod-us-east-2"],
    "version":           "v1alpha1",
    "registration":      "2026-05-17T18:42Z"
  }
}
```

The `keleustes.deploymentTargets` claim is what the hub-side
claim coordinator reads to decide whether an agent's `ClaimWork`
request is even allowed. Mismatched claim → rejected at the
coordinator, no NATS-side enforcement needed.

JWT rotation: agent re-fetches JWT via a hub-exposed
`/api/agents/<name>/jwt` HTTPS endpoint authenticated by the
NKey signature. Default rotation cadence 6 months; emergency
revocation is a CR field flip (`Agent.spec.revoked=true`) the
hub honors immediately by dropping the agent's NATS connection.

### 5.3 JetStream consumer group semantics for work claim

The actual work assignment (which agent gets which `SyncRun`
event for which `DeploymentTarget`) layers on top of two
JetStream primitives, used together:

1. **Claim** is on the `controller-locks` NATS KV bucket
   (SKA-324 §6). Key: `agent/<target-name>`. Value: the agent's
   `Holder` struct (PodName, NKey fingerprint, StartedAt).
2. **Work** is on the `keleustes-agent` stream (SKA-324 §5.1) —
   `WorkQueue` retention, so once a message is acked by one
   consumer it disappears. The agent uses a *durable consumer*
   bound to a subject filter that includes its claimed targets:
   `keleustes.agent.cluster.deploymenttarget.{prod-us-east-1,
   prod-us-east-2}.work`.

The hub publishes work for a `DeploymentTarget` on the matching
subject; whichever agent currently owns the target (per the KV
claim) consumes it (per the stream's WorkQueue semantics). Two
agents who somehow both claim the same target would also both
subscribe to the same work subject — but only one can ack each
message; the other's pull returns empty. The eventual
consistency window (≤ 30 s — the KV lease TTL) is bounded.

### 5.4 WebSocket-on-443 fallback

For environments where outbound TCP to non-443 ports is blocked
(restrictive corporate proxies, some PaaS networks), NATS leaf
supports WebSocket transport on TLS/443. The agent's
`ConnectConfig.PreferWebSocket=true` flag forces this path; the
default is auto-detect (try raw TCP, fall back).

The hub exposes both endpoints (`nats://hub:4222` and
`wss://hub:443/nats`) behind the same Gateway. WebSocket adds
some per-message framing overhead and a slight latency tax; the
trade is connectivity in places raw NATS can't reach.

When WebSocket is in use, `Status.LeafProtocol="ws"` is reported.

## 6. The `Agent` CR

ADR 0005 §168 promised this CR but did not specify the shape.
Here it is.

```go
// api/v1alpha1/agent_types.go
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=kag
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Targets",type=integer,JSONPath=`.status.claimedTargetCount`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Last Heartbeat",type=date,JSONPath=`.status.lastHeartbeat`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.agentVersion`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type Agent struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   AgentSpec   `json:"spec,omitempty"`
    Status AgentStatus `json:"status,omitempty"`
}

type AgentSpec struct {
    // NKey is the agent's NKey public key (the user-readable
    // form, e.g., "UAH42...."). Set at registration time; never
    // changed without revoking the agent.
    // +kubebuilder:validation:Pattern=`^U[A-Z2-7]{55}$`
    NKey string `json:"nkey"`

    // AllowedDeploymentTargets is the closed set of DeploymentTarget
    // names this agent may claim. Selector-based for the common
    // "this agent owns every target in region X" pattern.
    // +kubebuilder:validation:Required
    AllowedDeploymentTargets MemberSelector `json:"allowedDeploymentTargets"`

    // Region is informational metadata for placement / multi-region
    // routing. Mirrors the agent's pod region.
    // +optional
    // +kubebuilder:validation:MaxLength=253
    Region string `json:"region,omitempty"`

    // Revoked, when true, instructs the hub to drop the agent's
    // NATS connection on next observation and refuse re-handshake
    // until the field is reset. The agent's NKey remains in the
    // record (for audit forensics) but is non-functional.
    // +optional
    Revoked bool `json:"revoked,omitempty"`

    // JWTRotationCadence overrides the default 6-month JWT
    // rotation cadence. Set shorter for high-security tenants.
    // +optional
    // +kubebuilder:default="4380h"
    JWTRotationCadence metav1.Duration `json:"jwtRotationCadence,omitempty"`

    // RequiresStepUp reserves the MVP 4 field for tenant
    // isolation enforcement (per SKA-323 §3 / ADR 0004 §13).
    // Honored as a no-op until then.
    // +optional
    RequiresStepUp bool `json:"requiresStepUp,omitempty"`
}

type AgentStatus struct {
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    // AgentVersion is the version string the agent reports on
    // every heartbeat. Useful for fleet upgrade visibility.
    // +optional
    AgentVersion string `json:"agentVersion,omitempty"`

    // ClaimedTargets is the live list of DeploymentTarget names
    // the agent currently owns (from controller-locks KV).
    // +optional
    ClaimedTargets []string `json:"claimedTargets,omitempty"`

    // ClaimedTargetCount mirrors len(ClaimedTargets) for the
    // printer column.
    // +optional
    ClaimedTargetCount int32 `json:"claimedTargetCount,omitempty"`

    // LastHeartbeat is the agent's most recent heartbeat timestamp
    // (broker-recorded, not agent-clock).
    // +optional
    LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`

    // LeafProtocol reports whether the agent is connected via
    // raw TCP or WebSocket.
    // +optional
    // +kubebuilder:validation:Enum=tcp;ws;none
    LeafProtocol string `json:"leafProtocol,omitempty"`

    // JWTExpiresAt is when the agent's current JWT expires.
    // Surfaced for ops dashboards.
    // +optional
    JWTExpiresAt *metav1.Time `json:"jwtExpiresAt,omitempty"`

    // Conditions: Ready, Disconnected, Revoked, JWTExpiringSoon,
    // ClaimContention.
    // +optional
    // +listType=map
    // +listMapKey=type
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

**Validation webhook rules** (joining the existing webhook from
SKA-323 §10):

- `spec.allowedDeploymentTargets` cannot be unset (empty selector
  is fine — that means "this agent owns no targets, registered
  for future expansion" — but the field is required).
- `spec.nkey` cannot be reused across two `Agent` CRs (uniqueness
  index).
- `spec.region` must match the agent's reported region in
  status.agentVersion observation (advisory; warns on mismatch,
  does not reject).

**Status conditions:** `Ready` (connected + heartbeating),
`Disconnected` (no heartbeat in 2× expected interval),
`Revoked` (spec.revoked=true), `JWTExpiringSoon` (within 30 d
of expiry), `ClaimContention` (lease lost to another agent in
the last hour).

## 7. Pre-Registered Agent Flow

```
ADMIN                              HUB                            AGENT POD
  │                                 │                                │
  │  keleustesctl agent register \  │                                │
  │    --name=prod-us-east-7 \      │                                │
  │    --targets=prod-us-east-1,    │                                │
  │              prod-us-east-2 \   │                                │
  │    --region=us-east-1           │                                │
  │ ──────────────────────────────► │                                │
  │                                 │                                │
  │                                 │ 1. Generate NKey seed (locally) │
  │                                 │ 2. Create Agent CR              │
  │                                 │ 3. Sign initial JWT             │
  │                                 │ 4. Emit `agent-registered`      │
  │                                 │    audit event                  │
  │                                 │                                │
  │ ◄───── NKey seed + JWT ───────── │                                │
  │       (printed to stderr; CLI    │                                │
  │        sets file perms 0600)     │                                │
  │                                 │                                │
  │ Copy seed + JWT to Secret in    │                                │
  │ agent's namespace                │                                │
  │ ──────────────────────────────────────────────────────────────►   │
  │                                                                  │
  │                                                          5. Boot │
  │                                                                  │
  │                                                          6. Mount│
  │                                                             Secret│
  │                                                                  │
  │                                 │ ◄── 7. Connect (NKey+JWT) ──── │
  │                                 │                                │
  │                                 │ 8. Validate against Agent CR    │
  │                                 │ 9. Establish leaf connection    │
  │                                 │ 10. Open subscriptions          │
  │                                 │                                │
  │                                 │ ──── 11. Heartbeat OK ───────► │
  │                                 │                                │
  │                                 │ 12. ClaimWork(targets...)       │
  │                                 │ ──────────────────────────────► │
  │                                                                  │
  │                                                  13. Begin work  │
```

The `keleustesctl agent register` CLI does steps 1–4 in a single
call; the operator distributes the resulting Secret to the
agent's namespace (typical pattern: sealed-secrets, External
Secrets Operator, or per-tenant kubectl). The agent never sees
the registration UX; from its perspective, the Secret is just
there when it boots.

Re-registration (rotating NKey for a compromised agent): same
CLI, `--rotate` flag, generates a new seed + JWT but keeps the
`Agent` CR. Old NKey is moved to a `spec.revokedNKeys` history
list (not shown in §6 schema — added when rotation lands).
`agent-registered` audit event distinguishes initial vs.
rotation in the payload.

## 8. Work-Claim Protocol

### 8.1 Claim acquisition

```go
// internal/agent/coordinator.go (hub side, sketch)
func (c *Coordinator) HandleAgentConnect(ctx context.Context, agent *v1alpha1.Agent) error {
    for _, target := range c.resolveAllowedTargets(agent) {
        key := fmt.Sprintf("agent/%s", target.Name)
        _, err := c.kv.Create(key, c.holderForAgent(agent))
        if errors.Is(err, nats.ErrKeyExists) {
            // Another agent owns this target. Skip; the agent
            // will pick it up on the next claim sweep when the
            // current owner's lease expires.
            continue
        }
        if err == nil {
            c.markClaimed(agent.Name, target.Name)
        }
    }
    return nil
}
```

The hub does the claim on the agent's behalf — it is the
authoritative source of "which agent is allowed to own which
target" (via `AllowedDeploymentTargets`). The agent itself never
talks directly to the KV bucket; it just receives work on its
subscribed subjects once the hub has placed a claim.

Why hub-side claim? Three reasons:

1. **Authoritative permission check.** The hub knows the
   `AllowedDeploymentTargets` selector; checking on the agent
   side would require shipping the selector outcome down each
   handshake.
2. **Cleaner failover.** When agent A disconnects, the hub
   notices (heartbeat absence in `agent-presence` KV) and can
   proactively release A's claims, making them immediately
   available to B without waiting for the 30 s lease TTL.
3. **Audit centrality.** Every claim and release is a hub-side
   event, captured in SKA-322 §13.8's `agent-claimed-work` /
   `agent-dropped-work` verbs from the actor.type=system
   perspective.

### 8.2 Claim heartbeats

The agent's `Heartbeat()` call (every 10 s) carries:

- The agent's `Status.{AgentVersion, LeafProtocol,
  PendingPublishes, ActiveClaims}` snapshot — used to populate
  `Agent.status.*`.
- An ack that the agent is still actively processing its claimed
  targets.

The hub's claim coordinator processes the heartbeat by extending
all of that agent's KV claim leases. A missed heartbeat for 2
intervals (20 s) triggers a `Disconnected` condition; missed for
3 intervals (30 s — the lease TTL) releases the claims.

### 8.3 Claim release

```go
// graceful shutdown path on the agent
func (a *Agent) Shutdown(ctx context.Context) error {
    for _, c := range a.activeClaims {
        _ = a.transport.ReleaseClaim(ctx, c)  // best-effort
    }
    return a.transport.Disconnect(ctx)
}
```

Best-effort because in a forced-kill scenario (`SIGKILL`,
node failure) the lease TTL is the fallback. Reconcilers are
idempotent (CLAUDE.md guardrail); brief double-coverage during
failover is harmless.

## 9. Large-Payload Transport (NATS Object Store)

### 9.1 When to use it

Three cases warrant Object Store rather than inline NATS
messages:

1. **Initial cluster cache snapshot transfer.** When a freshly-
   started agent inherits a claim from a dead agent, it may
   benefit from a hub-cached snapshot of the target cluster's
   live state to skip the cold cache penalty. Snapshots are
   multi-MiB.
2. **Render-output handoff (alternate to direct agent render).**
   If a future mode has the hub render and the agent apply
   (rather than the agent rendering locally), the render result
   tarball moves through the bus.
3. **Diff results for large promotions.** A Promotion across
   200 targets generates a per-target diff report; the
   aggregated report can exceed the 8 MiB inline cap.

### 9.2 The Object Store bucket

```
bucket: keleustes-agent-payloads
ttl:    24 h  (transient — these are recomputable from state)
replicas: 3   (same as agent stream)
storage: file
max_bytes: 100 GiB  (operator-tunable)
```

Lifecycle is short — anything that needs to live longer goes to
the durable object-storage bucket (render cache, audit archive)
per SKA-324 §7.

### 9.3 Pointer round-trip

`StreamLargePayload` writes a blob and returns
`LargePayloadRef{Bucket, Key, Hash, Size}`. The reference is
embedded in a normal NATS message (small) to the recipient:

```json
{
  "subject":     "keleustes.agent.cluster.deploymenttarget.prod-us-east-1.snapshot",
  "payload": {
    "snapshot": {
      "bucket": "keleustes-agent-payloads",
      "key":    "snapshot/prod-us-east-1/01HQ8FRT9DC4VV1MX2N7K1P8YQ.tar.zst",
      "hash":   "sha256:9a8b...",
      "size":   12582912
    }
  }
}
```

The recipient calls `FetchLargePayload(ref)` to pull the blob.
Two RPCs (publish + fetch) instead of one (publish carrying the
blob), but no NATS-level fragmentation and no per-message size
limit pressure.

Hash verification on fetch is mandatory; mismatched hash means
the blob was rewritten during the lifecycle (shouldn't happen,
but cheap to catch).

## 10. Identity Propagation

When a SyncRun ultimately applies on the agent's cluster, the
audit event needs to record both:

- The `actor` that triggered the SyncRun (a human, a CI workload
  identity, or `system`).
- The `agent` that executed the work.

SKA-322 §6.5 handles this via `actor.delegatedFrom`. The
mechanism:

1. The originating actor's identity is set on the SyncRun's
   `metadata.annotations["keleustes.skaphos.io/originating-actor"]`
   when the SyncRun is created (by the SyncPlan controller).
2. The hub's claim coordinator carries this annotation into the
   work message it publishes to the agent.
3. The agent's audit emit for `sync.applied` sets
   `actor.type=agent`, `actor.subject="agent:<target>.<instance>"`,
   and `actor.delegatedFrom={<the originating actor block>}`.

The agent never has to authenticate the originating actor —
the hub already did, and the propagated annotation is a
post-authentication fact. The transport just carries a string.

## 11. Reconnection + Replay Semantics

### 11.1 Disconnect

Two flavors:

- **Graceful.** Agent calls `Disconnect()`, which releases
  claims, flushes pending publishes, sends a NATS close frame.
  Hub observes; emits `agent-disconnected` audit; releases any
  claims the agent didn't release itself.
- **Ungraceful.** TCP-level connection drop (network partition,
  pod evicted, hub leaf node restarted). Hub notices via missed
  heartbeats (per §8.2); 30 s grace window; then releases claims.

### 11.2 Reconnect

The NATS client library handles re-establishment automatically
when the connection drops. The agent's transport wrapper:

1. Detects the disconnection via the NATS client's `DisconnectErrCB`.
2. Logs at WARN; updates `Status.Connected=false`.
3. NATS client retries with exponential backoff (default 2 s →
   30 s max).
4. On reconnect, the agent re-attempts `ClaimWork` for each of
   the targets in its `keleustes.deploymentTargets` JWT claim.
   Targets that have been re-claimed by another agent in the
   meantime are skipped (the `Coordinator` rejects).
5. The durable JetStream consumer (per SKA-324 §8) preserves the
   cursor; missed messages during the disconnect are delivered
   on reconnect.

### 11.3 Replay correctness

Two messages that *would have been* delivered during the
disconnect window are delivered on reconnect. The agent's
reconciliation logic must be idempotent — applying the same work
twice produces the same outcome. This is the CLAUDE.md guardrail
plus the SKA-320 §9 invariant that render is a pure function;
the agent satisfies it by re-running the same `gitops-engine`
`pkg/sync` call, which is itself idempotent.

For ordering-sensitive sequences (multiple SyncRuns for the same
SyncPlan in close succession), the SyncRun's
`metadata.resourceVersion` is the agent's ordering primitive —
process in `resourceVersion` order, skip any with
`resourceVersion < last-processed`.

## 12. Backpressure

When the agent can't keep up — slow cluster apiserver, render
queue backed up, agent pod under memory pressure — three layers
push back:

1. **NATS subscription `Pending()`.** Each subscription has a
   bounded buffer; when the buffer fills, the NATS client
   applies flow control. The transport's `Status.PendingPublishes`
   surfaces this for operators.
2. **JetStream consumer `MaxAckPending`.** The agent's durable
   consumer has `MaxAckPending=64` (tunable). The hub stops
   delivering new messages once 64 are in-flight without ack.
   Backpressure naturally bounds.
3. **Heartbeat-with-load-signal.** The heartbeat payload carries
   `pendingPublishes` and `activeClaims`. The hub's claim
   coordinator, observing sustained high pending counts, can
   refuse new claims for that agent (`Agent.status.conditions`
   gains `BackpressureActive`).

A `BackpressureActive` agent for >5 minutes triggers an alert.
The runbook is "investigate the agent's cluster" — backpressure
is a symptom, not a cause.

## 13. Failure Modes

| Failure                                    | Behavior                                                                                                                                  |
|--------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------|
| Single agent pod loss                       | Lease expires in ≤ 30 s (claim coordinator notices via heartbeat absence). Another agent in the `AllowedDeploymentTargets` selector pool claims. Cluster cache rewarms on the new owner (one-time cost). |
| All agents for a target offline             | Target's work piles up in `keleustes-agent` stream (WorkQueue retention is 1 h per SKA-324 §5.1). Within 1 h: any agent that comes online claims; processing resumes. After 1 h: oldest work drops per `discard:old`. Alert at 30 min unprocessed. |
| Agent's JWT expires (rotation missed)        | Agent's NATS connection drops on next reconnect attempt (JWT validation fails). Hub emits `Agent.status.conditions[Disconnected].reason=JWTExpired`. Operator runs `keleustesctl agent register --rotate`; new Secret distributed; agent reconnects. |
| Agent NKey compromised                      | Operator sets `Agent.spec.revoked=true`. Hub observes within one reconcile (≤ 5 s); drops the connection; rejects re-handshake. Old NKey moved to `spec.revokedNKeys`. New agent registered if replacement needed. |
| NATS leaf endpoint unreachable from agent   | NATS client retries with exponential backoff. Agent's local controller continues with cached state until reconnect. Cluster reconciliation pauses for the disconnect duration; on reconnect, replay catches up. |
| Hub-side claim coordinator down             | Existing claims continue (lease TTL holds for 30 s without coordinator action); new claims pause. SKA-328 single-leader fallback on the coordinator means a sibling pod takes over quickly. |
| Cluster cache OOM on agent                  | gitops-engine `pkg/cache` watches abort; agent pod's memory exhausted. Pod restarts (Kubernetes OOMKill); claim lease expires; new pod takes the target. Operator response: bump agent pod memory limit per SKA-326 §11.2 fleet sizing recommendations. |
| Two agents both think they own target X     | Brief — bounded by KV lease TTL. NATS WorkQueue retention ensures only one acks each message; the duplicate observes empty pulls. Idempotent reconciler closes the gap on the actual K8s side. |
| Hub claim coordinator double-grants         | Detection: agent receives two consecutive `claim-granted` for the same target. Reaction: agent ignores the second (already owns), logs, emits an audit event with verb `agent-claim-anomaly`. Investigation manual; usually indicates a coordinator bug or a clock skew. |

## 14. Alternate-Transport Sketches (Generality Test)

The interface (§4) is implementation-agnostic. Two alternates
proven possible by sketch — neither implemented in MVP 2, but
the interface must remain stable when they land.

### 14.1 gRPC bidirectional streaming (SKA-378, MVP 3)

```go
// internal/agent/transport/grpc/grpcclient.go (sketch)
type GRPCTransport struct {
    conn   *grpc.ClientConn
    stream pb.AgentBus_StreamClient
    // ...
}

func (g *GRPCTransport) PublishEvent(ctx context.Context, env EventEnvelope) error {
    return g.stream.Send(&pb.AgentMessage{
        Type: pb.AgentMessage_PUBLISH,
        Subject: env.Subject,
        Payload: env.Payload,
        MsgId:   env.MsgID,
    })
}

func (g *GRPCTransport) ClaimWork(ctx context.Context, key string) (Claim, error) {
    resp, err := g.client.ClaimWork(ctx, &pb.ClaimRequest{Key: key})
    if err != nil { return nil, err }
    return &grpcClaim{key: key, expiresAt: resp.ExpiresAt.AsTime()}, nil
}

// Heartbeat, StreamLargePayload, Subscribe etc. map 1:1 onto
// gRPC service methods backed by hub-side proto definitions.
```

Why gRPC matters: some enterprise customers reject NATS
operationally (it's another distinct system to learn). gRPC
over TLS reuses standard HTTP/2 infrastructure, integrates with
standard service meshes, and benefits from existing IAP
gateways.

Trade-off: gRPC bidirectional streams need a long-lived
HTTP/2 stream which not all proxies preserve correctly. We
maintain NATS leaf as the default specifically because of this
proxy fragility.

### 14.2 HTTP/2 long-poll (extreme-environment fallback)

```go
// internal/agent/transport/longpoll/longpoll.go (sketch)
type LongPollTransport struct {
    httpClient *http.Client
    baseURL    string
    // ...
}

func (l *LongPollTransport) Subscribe(ctx context.Context, subject string, handler Handler) (Subscription, error) {
    sub := &longPollSubscription{handler: handler, done: make(chan struct{})}
    go func() {
        for {
            select {
            case <-sub.done: return
            default:
                // GET /api/agent/poll?subject=<subject>&waitMax=30s
                // Returns 200 with messages, or 204 on timeout.
                msgs, _ := l.fetchOne(ctx, subject)
                for _, m := range msgs {
                    _ = handler(ctx, m)
                }
            }
        }
    }()
    return sub, nil
}
```

Long-poll is the lowest-common-denominator alternative —
works through any proxy that forwards HTTP/2 (or HTTP/1.1 with
keep-alive). Drawback: per-message latency floor of half the
poll interval. Useful for explicitly-degraded-mode agents in
environments where neither raw TCP nor WebSocket nor gRPC works.

Not in any current MVP roadmap; included to prove the interface
hasn't accidentally pinned itself to push-style transports.

## 15. Per-MVP Timeline

| MVP | Ships                                                                                                         |
|-----|---------------------------------------------------------------------------------------------------------------|
| 0   | Nothing — agents are an MVP 2 feature.                                                                         |
| 1   | The `internal/agent/transport.Transport` interface lands; `transport/faketransport/` ships for use in benchmark mock-agent (SKA-326 §6.1) and unit tests. No real NATS implementation yet — MVP 1 sync work is hub-only. |
| **2** | NATS leaf reference implementation (`transport/leaf/`) ships. `Agent` CR + `keleustesctl agent register` CLI lands. Production agents claim DeploymentTargets and execute SyncRuns. SKA-363's scope. |
| 3   | SKA-378 (optional gRPC transport) lands as a second implementation. WebSocket-on-443 path moves to first-class. JWT rotation tooling matures.                                                                          |
| 4   | Tenant-isolation enforcement (RequiresStepUp on Agent CR; SKA-323 §3 / ADR 0004 §13) goes live. Per-tenant NATS account scoping added.                                                                                   |

## 16. Open Questions

1. **JWT issuer key management.** The hub signs agent JWTs with
   a hub-private key; the agent verifies with the hub's public
   key. Today the public key is bundled in the agent's Secret
   alongside the NKey seed. Open: should the key rotate, and if
   so, how does the agent discover the new public key? Probably:
   `.well-known/jwks` endpoint on the hub, with `kid` claim
   selecting which key signed each JWT. Defer concrete design
   to MVP 3.

2. **Multi-region claim affinity.** An agent in `us-east` can
   claim a `DeploymentTarget` in `us-west`. Should the claim
   coordinator prefer same-region matches when multiple agents
   are eligible? Likely yes for latency, but reduces failover
   flexibility. Confirm with MVP 3 multi-region benchmark.

3. **Multi-agent-per-target ("hot standby") pattern.** Today
   strict 1:N. A future high-availability mode might allow N
   agents to claim a target, with one active and N-1 standby
   (zero failover latency at cost of cluster-cache duplication).
   Defer until a real customer requirement arrives.

4. **Bulk-claim API.** An agent owning 50 targets currently does
   50 separate `ClaimWork` calls on connect. A `ClaimWorkBatch`
   verb would collapse this. Trivial optimization; defer until
   the per-connect latency is measured at MVP 2 benchmark scale.

5. **Cross-cluster work-stealing for emergencies.** When a region
   loses all its agents, the work piles up. Today no automatic
   cross-region claim — operator intervention required.
   `PromotionPolicy.spec.crossRegionFallback=true` could opt in,
   at the cost of latency spikes. Discussion deferred until MVP
   3 multi-region behavior is observed.

## 17. Compliance with Prior Decisions

| Decision                                          | This plan honors it by                                                                                                                |
|---------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------|
| ADR 0005 §5 (NATS leaf default, pluggable)        | NATS leaf is the only MVP 2 implementation; the interface is generic enough that §14.1 gRPC and §14.2 long-poll fit without changes.   |
| ADR 0005 §7.3 (NKey + JWT, outbound-only)         | NKey+JWT auth flow specified in §5.2; outbound-only enforced architecturally in §5.1 (no hub→agent connection).                       |
| ADR 0005 §11.5 (DeploymentTarget ownership)       | Deterministic 1:N agent:target claim via the same `controller-locks` NATS KV bucket — preserves cluster-cache locality.               |
| ADR 0005 §168 (Agent is a CR)                     | The `Agent` CR is spec'd in §6 with kubebuilder validation, status conditions, printer columns, validation webhook rules.             |
| ADR 0006 §4 (containment rule)                    | The transport package lives in `internal/agent/transport/`; engine packages (`internal/sync/`, etc.) consume it via interface only.   |
| SKA-322 §6.3 (`actor.type=agent`)                 | `actor.subject` format `"agent:<target>.<instance>"` is the canonical form the agent emits.                                            |
| SKA-322 §13.8 (agent audit verbs)                 | `agent-registered` fires on admin's CLI action; `agent-claimed-work`, `agent-dropped-work`, `agent-disconnected` fire on coordinator events. |
| SKA-322 §6.5 (`actor.delegatedFrom`)              | §10 explains the propagation path from originating actor → SyncRun annotation → work message → agent's audit emit.                     |
| SKA-323 §3 (`IdentityProvider.kind=NATSNKey`)     | NKey-authenticated identities flow through this provider type; the agent's `actor.subject` lives there.                                |
| SKA-323 §12 (Agent not in RBAC alphabet)          | The `Agent` CR's create/delete is governed by native Kubernetes RBAC; no Keleustes-Role verbs apply to it.                            |
| SKA-324 §3.2 (`keleustes.agent.>` class)          | Every wire subject the agent uses derives from `internal/events/subject.For()` with class `agent` — no raw strings.                    |
| SKA-324 §5.1 (`keleustes-agent` WorkQueue, 1 h)   | The §13 "all agents for a target offline" failure mode behavior follows from this stream's WorkQueue semantics + 1 h retention.       |
| SKA-324 §6 (NATS KV bucket layout)                | Reuses `controller-locks` for claim and `agent-presence` for heartbeat surfacing. No new buckets.                                      |
| SKA-326 §6.1 (mock agent uses real `internal/events`) | The fake transport (`faketransport/`) implements this interface — the benchmark mock agent depends only on the interface, not NATS. |
| SKA-328 §5 (sharder NATS KV claim protocol)       | The agent claim protocol mirrors the sharded-controller claim protocol exactly — same KV bucket, same lease TTL, same CAS semantics.   |

## 18. Concrete Follow-ups

1. **SKA-363 (MVP 2 Agent v1)** — implements the NATS leaf
   transport against the interface defined here. Per-MVP timeline
   §15 row "MVP 2."
2. **SKA-378 (MVP 3 Optional gRPC Transport)** — second
   implementation per §14.1. Validates the interface generality
   in production code.
3. **New ticket: `keleustesctl agent` subcommand tree** —
   `register`, `rotate`, `revoke`, `status`, `list`. MVP 2 scope
   alongside SKA-363.
4. **New ticket: Agent CR scaffold** — add the `Agent` type to
   `api/v1alpha1/` per §6. MVP 1 work even though no real agent
   exists — the type needs to be installable so MVP 1's hub-side
   coordinator code compiles.
5. **New ticket: JWT signing key rotation tooling** — open
   question §16.1. MVP 3 timeline.
6. **New ticket: agent observability dashboard** — Grafana
   ConfigMap surfacing per-agent connection state, claim count,
   heartbeat age, JWT expiry. Lands alongside SKA-363; reuses
   the SKA-326 §9.3 dashboard infrastructure.
7. **Update `docs/DECISIONS.md`** — add this plan to the active
   interim contracts table (handled in the same commit as this
   plan).

---

**When this plan stabilizes** (after SKA-363 lands the NATS leaf
implementation and at least one real agent has run through MVP 2
benchmark) the plan promotes into a new ADR co-located with
ADR 0005 — likely ADR 0013 (Render → 0007, Audit → 0008, RBAC
shapes → 0009, JetStream layout → 0010, Sharded controllers →
0011, Benchmark harness → 0012, Agent transport → 0013). §16
open questions remain in this plan until resolved.
