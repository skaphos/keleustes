<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Git Mutation Attribution

- **Status:** Draft — 2026-05-18
- **Linear:** SKA-433 (this plan). Consumed by SKA-353 (Git Mutation Engine — GitHub provider), SKA-330 (IdentityProvider CRD — the `gitMutationAttribution` field lives there), SKA-432 (value-change Promotion — the most common producer of Git mutations).
- **Promotes into:** a future ADR co-located with ADR 0003 (Git invariant) once the GitHub provider has shipped both `bot-with-trailer` and `user-to-server` modes and a real customer has exercised the per-Project override path.
- **Related:** ADR 0003 (Git invariant), ADR 0004 (CRD-based RBAC), [audit-event-schema plan §6.5](./2026-05-audit-event-schema.md#65-actordelegatedfrom--system-on-behalf-of-human) (`actor.delegatedFrom`), [audit-event-schema plan §13.6](./2026-05-audit-event-schema.md#136-git-mutation) (Git-mutation verbs), [value-change Promotion plan §6](./2026-05-value-change-promotion.md#6-git-mutation-engine-handoff) (`MutationRequest` shape).
- **Out of scope:** the per-provider *implementation* tickets (GitLab/Azure DevOps/Bitbucket each get their own SKA-### later); SAML-SSO bridging between the customer's IdP and GitHub (the customer's GitHub admin configures that, Keleustes inherits the mapping).

## 1. Purpose and Scope

Every Git mutation Keleustes performs — a Promotion's PR, a value-change PR (SKA-432), a break-glass corrective PR (SKA-360) — has two attribution questions to answer:

1. **Who triggered it?** The audit-trail answer. `actor` in the Keleustes audit envelope. Always populated, always the human/system that requested the action.
2. **Who appears in Git as the author / PR opener?** The wire-side answer. Drives Git's `author`/`committer` fields, the PR's `user` field on GitHub, the email that lands in GitHub's audit log, the identity GitHub's branch protection sees.

The two are not the same in general. The trigger is alice@example.com authenticated to Keleustes via her OIDC IdP; the wire identity is whatever Git/GitHub will accept, which is bounded by the credentials Keleustes holds. OIDC tokens are not Git credentials — GitHub will not accept Alice's Okta access token as authorization to open a PR. A separate authorization mechanism is required.

This plan pins:

- The three attribution modes Keleustes ships — `user-to-server` (default), `bot-with-trailer`, and `service-account` — and the policy field that selects between them.
- The Keleustes GitHub App + how customers install it.
- Token storage, refresh, and revocation handling.
- The per-provider matrix (GitHub, GitHub Enterprise Server, GitLab, Azure DevOps, Bitbucket) where the same shape lands on each provider's auth model.
- The audit envelope's wiring — `actor.delegatedFrom` for the Mutation Engine's events; the `payload.actor.github` / `payload.actor.gitlab` sub-record on `git-pr-opened` per provider.
- Failure modes (token revoked mid-flight, user left, scope insufficient, refresh failed).

**Why this is load-bearing.** Get this wrong and one of three things happens. (a) PRs are authored by `keleustes[bot]` and a customer's compliance team comes back six months later asking "who actually authorized this push to prod?" The data is in our audit trail; it's not visible in GitHub's. (b) PRs are authored by alice but Keleustes is silently holding a 6-month refresh token for her — when she leaves the company, her token still works. (c) Customer's GitHub uses SAML SSO that auto-revokes Alice's tokens when her Okta session ends — Keleustes' refresh logic doesn't expect a hard revocation and starts opening PRs as `keleustes[bot]` without telling anyone. Each of these is a discrete failure mode the modes-and-handlers design below specifies.

## 2. Why a single attribution mode fails

Three classes of customer setup, none of which can be served by one fixed mode:

| Setup | Wants | Will not tolerate |
| --- | --- | --- |
| Strict-attribution enterprise (regulated industries, change-management heavy) | Every commit + PR shows the human's GitHub identity. GitHub audit log mirrors Keleustes audit log. | A bot identity opening PRs to prod even with the human named in the trailer — auditors don't read commit trailers. |
| Operator-light shop (small SRE team, lots of automation) | Most PRs are CI-triggered or scheduled; no per-user GitHub OAuth flows; one identity to mute / page. | Per-user token storage + refresh dance for every developer who occasionally pokes Keleustes. |
| Mixed-CI shop (humans + CI both trigger Git ops) | Human Promotions show the human; CI-triggered Promotions show a service-account identity (not the bot). | One global identity that obscures whether a 3am Promotion was human-Alice or CI-job-12345. |

The first wants `user-to-server`. The second wants `bot-with-trailer`. The third wants both, switching on the actor's `type` from the audit envelope. So Keleustes ships all three modes; the `IdentityProvider` CRD picks the default per cluster; per-Project / per-Application overrides handle the mixed-CI shop's need.

## 3. The three modes

### 3.1 `user-to-server` (default)

The user goes through GitHub's OAuth flow against the Keleustes GitHub App, granting Keleustes a user-to-server access token scoped to her account. The token authorizes Keleustes-acting-as-Alice to read/write repositories she can already read/write.

**Flow:**

1. Alice signs into Keleustes via her org's OIDC IdP (Okta etc.). Audit envelope's `actor` is Alice with her OIDC subject.
2. Alice triggers a Promotion (or any Git-mutating action) from the CLI/UI.
3. Promotion Engine reaches `MutatingGit` phase. The Git Mutation Engine looks up Alice's GitHub user-to-server token in the Keleustes token store (§6 below).
4. **First-time path:** no token found → Keleustes generates a cryptographically random, single-use nonce, stores it server-side (or in a signed/encrypted callback envelope) together with the initiating actor/session and the Promotion / return target, and redirects Alice to GitHub's authorization URL for the Keleustes App (`https://github.com/login/oauth/authorize?client_id=…&redirect_uri=…&state=<opaque-random-nonce>`). Alice consents; GitHub redirects back to Keleustes with a code and the same `state`; Keleustes validates that the nonce matches the initiating session/request before exchanging the code for an access token + refresh token; both are stored encrypted, scoped to Alice's actor subject.
5. **Subsequent path:** existing valid token found → used directly. Refresh token used if access token has expired.
6. The Git Mutation Engine opens the PR using Alice's token. Commit `author` is her GitHub identity (the email GitHub returns from `/user` for her); commit `committer` is the same; PR `user` field is her GitHub identity.
7. Audit envelope's `git-pr-opened` event records both Keleustes-side actor (Alice's OIDC subject) and GitHub-side actor (her GitHub login + id), tied by `requestId`.

**What Alice's permissions enforce.** GitHub denies the PR creation if Alice can't push to the branch (branch protection) or doesn't have write access to the repo. This is correct — Keleustes-as-Alice is bounded by Alice's actual permissions, not by the App's. The App's declared scopes are an upper bound: the App requests `contents: write` + `pull_requests: write` + `metadata: read`; Alice can grant less, never more.

**Token lifetimes (GitHub.com defaults):**

- User-to-server access token: 8 hours.
- Refresh token: 6 months (sliding window — every successful refresh resets the 6-month clock).
- Both are revoked together when Alice un-installs the App from her account, when she leaves the customer's org (SAML-bound accounts), or when GitHub's session-mapping mechanism (SAML SSO) decides her parent session expired.

### 3.2 `bot-with-trailer`

Same Keleustes GitHub App, but Keleustes uses **installation tokens** instead of user-to-server tokens. Installation tokens are server-to-server, scoped to the App's installation on the customer's org, no per-user flow.

**Flow:**

1. Alice triggers a Promotion.
2. Git Mutation Engine resolves the target repo's installation, mints an installation token (TTL 1h, auto-refreshed), uses it to open the PR.
3. The commit is authored by `keleustes[bot]@users.noreply.github.com` (the App's identity).
4. The actual user identity is recorded in:
   - Git commit's `Co-authored-by:` trailer:
     ```
     Co-authored-by: Alice Example <alice@example.com>
     ```
     where the email is Alice's provider-resolved GitHub account email, not her OIDC subject. GitHub recognizes `Co-authored-by:` for profile attribution only when that email is verified on her GitHub account; if Keleustes cannot resolve/store a verified GitHub email, it must still record Alice in audit and PR metadata but must not claim GitHub profile attribution from the trailer.
   - PR body header:
     ```
     **Triggered by:** alice@example.com (via Keleustes Promotion `payments/checkout-api-to-prod`)
     ```
   - PR label: `keleustes-triggered-by:alice` (sanitized, ≤50 chars to fit GitHub label-name limits, for filtering in the GitHub UI).
   - `metadata.annotations.keleustes.skaphos.io/git-actor` on the resulting Promotion CR for cross-system correlation.
5. Audit envelope's `git-pr-opened` event records the GitHub-side actor as `keleustes[bot]` and the `actor.delegatedFrom` as Alice — exactly the §6.5 pattern.

**When `bot-with-trailer` is the right pick:** small teams, CI-heavy automation, or customers where per-user GitHub OAuth is operationally expensive (e.g., they don't run SAML SSO and each user would have to maintain her own GitHub credentials).

### 3.3 `service-account`

A variant of `bot-with-trailer` where the bot identity is **per-Project**, not the global Keleustes App identity. The customer creates a dedicated GitHub user (e.g., `keleustes-payments@customer.example.com`) with its own SSH/HTTPS credentials and registers it in the Keleustes Project's `IdentityProvider` config. The Mutation Engine uses that identity for Git operations in the Project's repos.

**When this matters:** segregating CI-triggered Git ops from human-triggered ones, or satisfying a "no shared bot account across teams" compliance requirement. Tokens for service accounts are conventionally long-lived PATs or fine-grained PATs scoped to specific repos.

This mode is opt-in; not enabled by default since it requires customer setup outside Keleustes (creating the GitHub user, generating its credential).

### 3.4 Mode-selection precedence

When the Mutation Engine looks up the mode for a given Promotion:

1. Application's `spec.gitMutationAttribution.mode` if set.
2. Project's `spec.gitMutationAttribution.mode` (the Project CRD from ADR 0004) if set.
3. IdentityProvider's `spec.gitMutationAttribution.mode` (cluster-default).
4. Hardcoded fallback: `user-to-server`.

The actor's `type` (from the audit envelope) can override the mode:

- If `actor.type == "human"` and the mode resolves to `service-account`, the Engine ignores the service-account and falls back to `user-to-server` (a service account isn't the right wire identity for a human action).
- If `actor.type == "ci"` and the mode resolves to `user-to-server`, the Engine falls back to `service-account` if configured at the Project level, else `bot-with-trailer`.
- If `actor.type == "system"` (Keleustes acting on its own — e.g., scheduled drift-resolution), the Engine always uses `bot-with-trailer` regardless of the configured mode.

These overrides are policy, documented in this plan, and not customer-tunable in MVP 2. The reasoning: a human's identity is the wrong wire identity for a CI action; a CI's identity is the wrong wire identity for a human action; system actions need to be visibly system-authored.

## 4. The `IdentityProvider.spec.gitMutationAttribution` field

```yaml
apiVersion: keleustes.skaphos.io/v1alpha1
kind: IdentityProvider
metadata:
  name: okta-prod
spec:
  # ... existing OIDC fields per the rbac-crd-shapes plan ...

  gitMutationAttribution:
    # Cluster-default mode. Per-Project / per-Application overrides
    # allowed via the matching field on Project and Application.
    mode: user-to-server                  # | bot-with-trailer | service-account

    # GitHub App configuration. Required when any mode is selected for
    # any provider in the gitProviders list below.
    githubApp:
      # The App's GitHub-side App ID (numeric). Public — not a secret.
      appId: 123456
      # Reference to a Secret carrying the App's private key (PEM).
      # Keleustes uses the key to mint JWTs and installation tokens
      # for GitHub App authentication (for example in
      # bot-with-trailer mode).
      privateKeySecretRef:
        name: keleustes-github-app
        key: key.pem
      # Reference to a Secret carrying the App's webhook secret.
      # GitHub webhook deliveries are verified with this shared
      # secret via HMAC signature validation when webhook receivers
      # (SKA-357) come online.
      webhookSecretRef:
        name: keleustes-github-app
        key: webhook_secret
      # Public client ID used in user OAuth flows. Public.
      clientId: Iv1.0123456789abcdef
      # Client secret for the user OAuth flow (only needed when
      # user-to-server mode is enabled).
      clientSecretRef:
        name: keleustes-github-app
        key: client_secret
      # The URL the Keleustes UI/CLI redirects users to after they
      # complete the OAuth flow. Set to the Keleustes API server's
      # callback path.
      redirectURL: https://keleustes.example.com/api/v1/oauth/github/callback

    # Optional: per-Git-provider override of the global mode. The
    # Mutation Engine consults this when the target repo's host
    # matches one of the entries.
    gitProviders:
      - host: github.com
        mode: user-to-server
      - host: github.example.internal
        mode: bot-with-trailer            # GHES installation only
      - host: gitlab.example.internal
        mode: service-account
        serviceAccountRef:                # only used by service-account mode
          name: keleustes-gitlab-svc
          key: token

    # Token store config. Where Keleustes persists user-to-server
    # tokens (encrypted at rest). Per ADR 0005, JetStream-backed KV is
    # the runtime durable layer for hot indexes; per-user tokens go in
    # a dedicated `git-user-tokens` bucket with the Keleustes actor
    # subject as the key.
    tokenStore:
      backend: nats-kv                    # | secret-per-user (k8s Secrets)
      encryptionKeySecretRef:
        name: keleustes-token-encryption
        key: aes256.key
      # TTL for cached access tokens; refresh tokens have their own
      # provider-side TTL and are refreshed independently.
      accessTokenTTL: 8h

status:
  conditions: [ ... ]
  # When mode includes user-to-server, the status surfaces a count of
  # users with valid tokens so operators can see at a glance who's
  # opted in.
  authorizedUserCount: 47
```

The `gitMutationAttribution` block is optional only when no Git mutation is expected (read-only deployment). The CRD's validation webhook (`+kubebuilder:validation:XValidation`) checks that:

- If `mode` is `user-to-server` or any `gitProviders[*].mode` is `user-to-server`, `githubApp.clientSecretRef` is set.
- If `mode` is `service-account`, at least one `gitProviders[*].serviceAccountRef` is set.
- `redirectURL` is HTTPS (no plaintext for the OAuth callback).

## 5. The Keleustes GitHub App

One Keleustes GitHub App, published by Skaphos to the GitHub Marketplace, installed by each customer's GitHub admin. App permissions:

| Permission | Scope | Used for |
| --- | --- | --- |
| `contents` | `read` + `write` | Reading the config repo's current state; opening commits |
| `pull_requests` | `read` + `write` | Opening / reading / labeling PRs |
| `metadata` | `read` | Required by GitHub for any other permission; lets us list repos |
| `members` | `read` (organization-level) | Future: resolving GitHub usernames to Keleustes RBAC subjects (read-only; no membership-modify capability) |
| `actions` | (none) | Explicitly not requested — Keleustes doesn't run GitHub Actions |
| `administration` | (none) | Explicitly not requested — Keleustes never modifies repo settings |
| `secrets` | (none) | Explicitly not requested — Keleustes never reads or writes GitHub repo secrets |

User-scope (OAuth) permissions are the same set. The App's OAuth scopes are a subset of its installation scopes; the user-to-server mode inherits whichever is smaller in practice.

### 5.1 GitHub Enterprise Server

The App must be re-published on each customer's GHES instance (GitHub does not support cross-instance Apps). Skaphos provides a manifest-driven creation flow:

```bash
keleustesctl admin github-app create --gh-host=github.example.internal
# → prints the manifest URL the customer's GitHub admin clicks to install
#   the App on their GHES instance with Skaphos-recommended permissions.
```

The resulting App ID + client ID + client secret feed into `IdentityProvider.spec.githubApp` for that cluster.

### 5.2 What customers install vs. configure

| Setup step | Owned by | Where it lives |
| --- | --- | --- |
| Publish the Keleustes GitHub App on github.com | Skaphos | Once, in the Marketplace |
| Re-publish the App on a GHES instance | Customer GitHub admin (via `keleustesctl admin github-app create`) | Per-cluster |
| Install the App on the customer's GitHub org | Customer GitHub admin | One-time per customer |
| Wire App credentials into `IdentityProvider` | Customer platform team | Per-cluster |
| User-side OAuth authorization (when `user-to-server` mode) | Each user, first time they trigger a Git mutation | Per-user, sticky |

## 6. Token Storage, Refresh, Revocation

### 6.1 Storage

User-to-server tokens are encrypted with AES-256-GCM using a key in a Kubernetes Secret (`IdentityProvider.spec.gitMutationAttribution.tokenStore.encryptionKeySecretRef`). The encrypted blob is stored in NATS KV bucket `git-user-tokens` keyed by `<actor.subject>` (Alice's OIDC subject, e.g., `okta|01HQ7…`).

The KV bucket replication and persistence semantics are SKA-324's: 3x replica, file storage, KV (not stream) since we only need point-lookup access. The encryption key is rotated on a Skaphos-driven schedule (recommended quarterly); rotation re-encrypts every token in a single pass without invalidating them.

Service-account credentials (long-lived) are stored as Kubernetes Secrets per the standard pattern — `IdentityProvider.spec.gitMutationAttribution.gitProviders[*].serviceAccountRef`. Kubernetes RBAC + the operator's namespace-scoped read permission limits who can extract them.

Installation tokens are minted on-demand (every `MutatingGit` reconcile) from the App's private key + the installation ID. They're held in-memory for their 1-hour lifetime and never written to durable storage. The App private key is the only durable credential for `bot-with-trailer` mode.

### 6.2 Refresh

User-to-server tokens carry both an access token (8h TTL) and a refresh token (~6 month sliding TTL). The Mutation Engine refreshes on access:

```
on user-to-server token use:
  if access_token.expires_at - now() < 5min:
    new_access, new_refresh = github.refresh(refresh_token)
    store new_access, new_refresh atomically under <actor.subject>
    use new_access
```

The 5-minute buffer avoids the race where the Engine starts a multi-second Git operation with a 30-second-old token that expires mid-flight. Refresh failures fall through to a re-authorization prompt (§6.3).

Installation tokens are minted fresh every reconcile; no refresh state machine needed.

### 6.3 Revocation + the "user left the company" flow

Three revocation events to handle, in increasing severity:

1. **Token expired and refresh fails.** Most commonly: refresh token TTL exceeded. The Mutation Engine logs an audit event (`git-mutation-failed` with reason `RefreshRevoked`), surfaces a `Reason: ReauthorizationRequired` condition on the Promotion CR, and waits for Alice to re-trigger from the CLI/UI (which kicks off a fresh OAuth flow).

2. **User revoked the App authorization.** Alice goes to her GitHub user settings → Applications → Keleustes → Revoke. Subsequent Mutation Engine calls return 401; we treat as case 1.

3. **User account suspended / departed.** Customer's GitHub admin suspends Alice's account, or SAML-SSO mapping is revoked. Same surface — 401 from GitHub. The Engine emits an audit event with `Reason: UserSuspended` and the operator's IT team takes it from there.

The Engine **does not** silently fall back to `bot-with-trailer` when a user-to-server token is unavailable. That fallback would muddy the audit trail (alice's Promotion would suddenly appear as bot-authored without a clear reason). Instead the Promotion blocks with a clear condition.

### 6.4 Token-store backend choices

`tokenStore.backend` accepts:

- `nats-kv` (default, post-MVP 1): the bucket described above. Lives where the rest of Keleustes' hot index data lives.
- `secret-per-user` (MVP 2 starter): one Kubernetes Secret per user, named `keleustes-git-token-<sha256(actor.subject)[0:16]>`. Simpler to bootstrap (no NATS KV dependency); operationally heavier at customer scale (5000 users = 5000 Secrets). Recommended only for MVP 2 demos; MVP 3 customers migrate to `nats-kv`.

## 7. Per-provider matrix

The same three-mode shape lands on each provider; the implementation differs.

### 7.1 GitHub.com / GitHub Enterprise Server

Detailed in §3–§6 above. The reference implementation.

### 7.2 GitLab.com / GitLab self-hosted

GitLab has the closest GitHub-equivalents:

- **OAuth Apps:** map to `user-to-server`. Each user authorizes the Keleustes OAuth App; tokens are user-scoped, refreshable (default TTL 2h access, 1 month refresh).
- **Project access tokens / Group access tokens:** map to `service-account`. Customer creates a token per Project (or Group) and stores it in the Keleustes Secret referenced in the IdentityProvider.
- **GitLab applications cannot mint per-installation tokens** the way GitHub Apps can. The `bot-with-trailer` equivalent is a single dedicated GitLab user (`keleustes-bot@customer.example`) with its own PAT — operationally equivalent to a service account, just one tier higher in the customer's directory.

MVP 3 ticket lands the GitLab provider.

### 7.3 Azure DevOps

Two practical paths:

- **Azure AD federation:** the Keleustes operator runs with a Service Principal in the customer's Azure AD tenant; Azure DevOps trusts the SP. Effectively a `service-account` mode. Tokens come from the standard Azure SDK auth flow (workload identity in-cluster).
- **PATs:** legacy fallback. Customer creates a PAT per user (for `user-to-server`) or a single PAT (for `service-account`).

ADO doesn't have a GitHub-App equivalent for per-installation tokens. The default mode for ADO is `service-account` via the SP path.

### 7.4 Bitbucket Cloud / Bitbucket Data Center

Bitbucket has OAuth 2.0 and app passwords. The `user-to-server` mode maps to OAuth; `service-account` maps to a dedicated user with an app password; `bot-with-trailer` is again equivalent to a service account from Bitbucket's perspective.

### 7.5 Provider abstraction at the code layer

The Mutation Engine exposes an interface:

```go
// in internal/mutation/provider.go (MVP 2)
type GitProvider interface {
    // Resolve a target repo's owning installation/identity given the
    // mode + IdentityProvider config.
    ResolveActor(ctx context.Context, repo GitRepoRef, mode AttributionMode, actor AuditActor) (WireActor, error)

    // Open a PR with the resolved wire-side identity.
    OpenPR(ctx context.Context, req MutationRequest, wire WireActor) (PRRef, error)

    // Refresh a long-lived credential (used by user-to-server's
    // refresh-token flow; no-op for service-account).
    RefreshIfNeeded(ctx context.Context, wire *WireActor) error

    // Report the OAuth authorization URL for first-time user enrollment.
    AuthorizeURL(actor AuditActor, returnTo string) (string, error)
}
```

Each provider implements this. The Mutation Engine never speaks GitHub-specific or GitLab-specific shapes directly.

## 8. Audit Envelope Wiring

The audit envelope already supports the delegated-actor pattern (§6.5 of audit-event-schema). This plan extends it concretely for Git mutations.

### 8.1 `actor.delegatedFrom` on Mutation Engine events

When the Mutation Engine emits a `git-pr-opened` (`git.pr.opened.v1`) event, the envelope's `actor` block is the **wire identity** Keleustes used — for `user-to-server` it's Alice's GitHub identity, for `bot-with-trailer` it's `keleustes[bot]`, for `service-account` it's the configured SA. The `actor.delegatedFrom` block records the **Keleustes-side trigger** — Alice's OIDC subject in all three cases.

```jsonc
{
  "actor": {
    "type": "agent",                          // or "system" for bot mode
    "subject": "github:alice",                // wire-side identity
    "subjectId": "github|12345678",
    "identityProvider": "github-app:keleustes",
    "delegatedFrom": {
      "type": "human",
      "subject": "alice@example.com",         // Keleustes-side trigger
      "subjectId": "okta|01HQ7…",
      "identityProvider": "okta-prod"
    }
  },
  "action": { "verb": "git-pr-opened", ... },
  "payload": { "@type": "git.pr.opened.v1", "url": "https://github.com/...", "mode": "user-to-server" }
}
```

For `bot-with-trailer`:

```jsonc
{
  "actor": {
    "type": "system",
    "subject": "keleustes-mutation-engine",
    "delegatedFrom": { /* alice's actor */ }
  },
  "payload": {
    "@type": "git.pr.opened.v1",
    "url": "...",
    "mode": "bot-with-trailer",
    "wireActor": "keleustes[bot]",            // explicit for SIEM consumers
    "coAuthors": ["alice@example.com"]
  }
}
```

The `payload.mode` field is the audit consumer's signal for "which attribution mode did this Mutation use" — important for compliance reporting that filters on it.

### 8.2 New payload fields (additive to `git.pr.opened.v1`)

The `promote.v1` payload from SKA-432 carries the Promotion's content. The `git.pr.opened.v1` payload (§13.6 of audit-event-schema) gains three additive fields:

- `mode`: `user-to-server` | `bot-with-trailer` | `service-account` (string).
- `wireActor`: the GitHub/GitLab/etc. identity that appears on the PR (string).
- `coAuthors`: list of audit subjects recorded in `Co-authored-by:` trailers (string array; only populated for `bot-with-trailer` and `service-account`).

Additive per §5.1 of audit-event-schema; no `schemaVersion` bump.

## 9. Failure Modes

| Failure | Mode-specific behavior | Operator surface |
| --- | --- | --- |
| User token expired AND refresh failed | `user-to-server`: Promotion → `Failed: ReauthorizationRequired`. `bot-with-trailer`: n/a (no user token). | Promotion condition + audit event; UI prompts re-auth |
| User revoked the App | Same as above (401 from GitHub indistinguishable) | Same |
| User account suspended | Same path, but the audit event's reason is `UserSuspended` (set when GitHub returns a specific error code, otherwise generic `ReauthorizationRequired`) | Surface to IT/SecOps |
| GitHub App scope insufficient | All modes: PR creation returns 403; Promotion → `Failed: InsufficientScope` | Audit event names the missing scope; operator updates the App permissions in GitHub admin UI |
| App private key compromised | All modes: rotate via `IdentityProvider.spec.gitMutationAttribution.githubApp.privateKeySecretRef` swap; existing installation tokens expire within 1 hour, refreshed against the new key | Operator alert; audit event names the rotation |
| NATS KV unavailable (tokenStore.backend=nats-kv) | `user-to-server`: Promotion waits in `Blocked: TokenStoreUnavailable`. `bot-with-trailer` + `service-account` unaffected (don't use KV). | Surface as Promotion blocker; depends on broader NATS recovery |
| Branch protection denies the push | All modes: PR open succeeds but is unmergeable; `WaitingForMerge` phase stays until human resolution or Promotion is cancelled | Surface in the Promotion's PR-status condition |
| User has no GitHub identity at all (CI-triggered, no SAML mapping) | `user-to-server`: falls through to `bot-with-trailer` per §3.4 actor-type override | Audit event records the fallback |
| Refresh-token theft | Worst case: attacker uses stolen refresh token to mint access tokens for up to 6 months. **Mitigation:** rotation of the token encryption key invalidates all stored refresh tokens at once; UI surfaces "active sessions" with last-used timestamps per the GitHub App audit log | Customer-facing UI for active-session revocation |

## 10. Compliance with Prior Decisions

| Decision | This plan honors it by |
| --- | --- |
| ADR 0003 (Git invariant) | Every Git mutation goes through a Mutation Engine PR; no live-cluster mutation path is added. Auth mode selects the *attribution*, never bypasses the Git boundary. |
| ADR 0004 (CRD-based RBAC) | `IdentityProvider`, Project, and Application carry the `gitMutationAttribution` field; Keleustes' Role/RoleBinding gate who can edit those fields. |
| ADR 0005 (no RDBMS) | Token storage uses NATS KV (default) or Kubernetes Secrets — both ADR-0005-compliant. No SQL anywhere in the token-management path. |
| Audit-event-schema plan §6.5 (`actor.delegatedFrom`) | Used exactly as designed — the Mutation Engine's events carry the wire-side identity in `actor` and the Keleustes-side trigger in `actor.delegatedFrom`. |
| Audit-event-schema plan §13.6 (Git mutation verbs) | New `mode`/`wireActor`/`coAuthors` fields added additively to `git.pr.opened.v1` per §5.1 (additive forever). No schemaVersion bump. |
| Audit-event-schema plan §8.2 (Redaction) | App private keys, OAuth client secrets, encryption keys, and user tokens never appear in audit payloads. The redaction-rule list gains `*.tokenStore.encryptionKey`, `*.clientSecret`, `*.privateKey` patterns. |
| Value-change Promotion plan §6 (Mutation Engine handoff) | The `MutationRequest` interface gains an `AttributionMode` field (resolved by the Promotion Engine per §3.4 precedence) which the Engine consults when picking a wire identity. |

## 11. Open questions

1. **First-time-OAuth UX from the CLI.** The web UI redirect flow is straightforward; from `keleustesctl set` or `keleustesctl promote`, what happens on first use? Options:
   - Print a URL + a one-time code (GitHub's device flow) — works in headless contexts.
   - Open a browser via `xdg-open` / `open` — works on developer laptops, fails in CI.
   - Refuse with a clear error pointing at the web UI.
   Probably the device flow as default, with `--no-browser` falling through to print-the-URL. Confirm before MVP 2.

2. **SAML-bound GitHub identities and Keleustes' subject normalization.** When a customer uses GitHub Enterprise Cloud with SAML-SSO, Alice's GitHub login (`@alice-customer`) is bound to her SAML NameID (her Okta subject). Keleustes' audit envelope's `actor.subject` is the Okta subject. We need a deterministic way to map between them — probably during the OAuth callback, Keleustes reads GitHub's `/user/saml-identity` (where available) and stores the mapping. Detail to confirm during GitHub App design.

3. **Token store backend default for MVP 2 vs. MVP 3.** The plan defaults to `nats-kv` but MVP 2's NATS KV story (SKA-365) lands alongside the Mutation Engine. Acceptable race? If NATS KV isn't ready when the Mutation Engine ships, fallback default is `secret-per-user`. Coordinate dates.

4. **`bot-with-trailer` Co-authored-by trailer character limits.** Git's trailer parsing is permissive but GitHub renders only the first 255 chars per trailer. Alice's display name + email is well under that, but a Promotion with 50 reviewers in the `Co-authored-by:` list would exceed it. Probably need to cap at the first 10 distinct co-authors and add a note in the PR body.

5. **Service-account credential rotation.** Long-lived PATs (GitLab, Bitbucket) need rotation. The IdentityProvider could carry a `rotationPolicy` field — but that adds operator-side machinery. For MVP 2 we ship "manual rotation: update the Secret, the Engine picks up on next reconcile." For MVP 3 consider scheduled rotation via the Promotion Engine.

6. **Rate-limiting against GitHub.** GitHub.com's primary rate limit on user-to-server tokens is 5000 requests/hour per user. A Promotion produces ~5-10 API calls (list, get-branch, create-blob, create-tree, create-commit, create-PR, add-labels, request-reviewers). 5000 / 10 = 500 Promotions/hour/user, which is more than any human will trigger. Confirm with realistic CI-load profiles before MVP 3.

7. **`gitMutationAttribution` overrides per Application.** §3.4 specifies the precedence chain. Edge case: an Application-level override changes mode between two Promotions — does the second one's "user-to-server" token from the first still apply? Yes, tokens are user-scoped, not Promotion-scoped; the Engine just doesn't use them when the override says service-account. Document explicitly.

## 12. Phased Rollout

| MVP | Work in this plan's scope |
| --- | --- |
| **MVP 2** | The Mutation Engine ships with `bot-with-trailer` and `user-to-server` modes for **GitHub.com only** (GHES + other providers follow). IdentityProvider gains the `gitMutationAttribution` field. Token store backend defaults to `secret-per-user` (NATS KV migration ships when SKA-365 lands). Web-UI OAuth callback wired; CLI uses device flow. Audit-envelope wiring for the wire-identity / delegated-from split lands. Per-Project + per-Application overrides functional. |
| **MVP 3** | GitHub Enterprise Server support (per-customer App publishing). GitLab provider (OAuth + Project access tokens). Azure DevOps provider (Service Principal default + PAT fallback). Token store migration to NATS KV (SKA-365). Bot-with-trailer's `Co-authored-by:` cap. Rate-limit observability dashboard. |
| **MVP 4** | Bitbucket provider. Scheduled service-account credential rotation. UI for per-user active sessions (revoke a user's token from the operator console). SIEM-export of `git-pr-opened` events with the new payload fields. |

## 13. Concrete Follow-ups

1. **SKA-### — `IdentityProvider.spec.gitMutationAttribution` schema** (MVP 2). Extend the IdentityProvider CRD (SKA-330) with the field tree from §4 above. Schema-only ticket; reconciler validation comes next.

2. **SKA-### — Token store interface + `secret-per-user` backend** (MVP 2). Define the `TokenStore` interface; ship the Kubernetes-Secret-per-user implementation. NATS-KV backend lands in step #5.

3. **SKA-### — Keleustes GitHub App + bootstrap flow** (MVP 2). Publish the App to the GitHub Marketplace. Add `keleustesctl admin github-app create` for GHES customers per §5.1. Document the customer-side install flow.

4. **SKA-### — Mutation Engine GitHub provider with mode selection** (MVP 2). The `GitProvider` interface (§7.5), the GitHub implementation, the mode-precedence resolution (§3.4), the OAuth callback handler, the refresh logic.

5. **SKA-### — NATS-KV token store backend** (MVP 3). Migrate from per-user Secret to KV-bucket storage. Companion to SKA-365 (NATS KV for hot indexes).

6. **SKA-### — Audit payload schema amendment** (MVP 2). Additive update to `git.pr.opened.v1` (and `git.pr.merged.v1`, `git.mutation.failed.v1`) for the new `mode`/`wireActor`/`coAuthors` fields. Audit-event-schema plan §13.6 gets the new field documentation.

7. **SKA-### — UI re-auth prompt** (MVP 2). When a Promotion is blocked on `ReauthorizationRequired`, the UI surfaces a "Re-authorize Keleustes for GitHub" button that kicks off a fresh OAuth flow and resumes the Promotion.

8. **SKA-### — CLI device-flow OAuth bootstrap** (MVP 2). `keleustesctl auth github` runs the device flow and stores the resulting token. Documented in §11 open question 1.

9. **PROPOSAL.md and CLAUDE.md** — cross-link this plan from PROPOSAL §14 (Git mutation) during the next refresh.

10. **DECISIONS.md** — add a row in "Plans that have not yet stabilized"; promote to an active interim contract once SKA-### implementation tickets above land their first reconciler scaffolds.

---

**When this plan stabilizes** (after the GitHub provider ships both modes in MVP 2 and a real customer has exercised the per-Project override path), §1–§10 promote into a new ADR co-located with ADR 0003 (since this plan's core contribution is extending the Git-invariant machinery to handle delegated authority). The provider-specific implementations stay in working material; the three-mode contract becomes the durable record.
