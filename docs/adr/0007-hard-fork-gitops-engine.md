<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# ADR 0007 — Hard-fork `gitops-engine` into `skaphos/gitops-engine`

- **Status:** Accepted — amended 2026-05-18 (friendly-fork posture clarification — see Amendments below)
- **Date:** 2026-05-17
- **Deciders:** Platform Architecture (Skaphos)
- **Linear:** SKA-430 (extraction execution), SKA-421 (k8s.io ceiling lift, rescoped against the fork)
- **Related:** ADR 0006 (Engine boundaries and `gitops-engine` reuse), ADR 0005 (Distributed runtime — no-RDBMS demand), `docs/plans/2026-05-gitops-engine-spike.md` (SKA-327 spike empirical findings)
- **Supersedes:**
  - ADR 0006 "2026-05-17 (afternoon) — Soft-fork strategy abandoned" amendment's *Decision* paragraph that froze the k8s.io ≤ v0.34 ceiling as a steady-state constraint. That framing assumed staying on vanilla upstream; this ADR moves to a Skaphos-owned fork instead.
  - ADR 0006 §4's implicit assumption that the canonical import path is `github.com/argoproj/argo-cd/gitops-engine`. Under this ADR the canonical path becomes `github.com/skaphos/gitops-engine`.

## Amendments

### 2026-05-18 — Friendly-fork posture clarification

The original §3 ("Backport workflow") framed the fork as one-directional: backports come down from argoproj; Skaphos changes do not ship back up. Operator override during the SKA-430 extraction reversed that framing — `skaphos/gitops-engine` is a **friendly fork** that actively maintains the intent to upstream Skaphos-originated work.

**What changed in the shipped fork:**

- `README.md` and `NOTICE` call the repo a *friendly fork* rather than a divergent one.
- The fork ships an [`UPSTREAMING.md`](https://github.com/skaphos/gitops-engine/blob/main/UPSTREAMING.md) tracking outbound PRs in three stages — Pending → Submitted → Merged. The first row, currently in *Pending outbound*, is commit `eb643e2 chore(gitops-engine): drop dead autoscaling/v2beta1 and v2beta2 references`, the cleanup carried forward from the SKA-327 spike.
- The `UPSTREAMING.md` author checklist requires every Skaphos commit be shaped for verbatim (or near-verbatim) upstream PR submission: atomic, upstream-style subject lines, no Skaphos-only jargon, no Linear-only references. Commits that genuinely can't be upstreamed (Skaphos-specific abstractions in shared paths) get a tracking row explaining why.

**What is unchanged:** §1 (extraction sequence — already executed under SKA-430), §2 (Apache-2.0 license + NOTICE attribution stanza), §4 (containment rule), §5 (ceiling-lift mechanism — SKA-421 still lands on the fork first for unblocking). The fork's *existence* is still justified by the maintenance-velocity worry from §3 and the SKA-327 spike. The amendment changes the **intent** behind the fork, not the **mechanism**.

**Why the change.** The hard-fork framing in the original §3 was a tactical response to maintenance-velocity worry, not a position on the Argo ecosystem. The friendly-fork posture preserves the unblock-now benefit while keeping the door open to feeding improvements back upstream when they're ready. It also reduces the risk that the fork drifts into a permanent silo of Skaphos-only changes: the discipline of "shape every commit so it could be upstreamed verbatim" is a forcing function for keeping the divergence small.

**Compliance updates carried by this amendment:**

- §3's first sentence below ("The fork does **not** ship patches back to argoproj") is **superseded** by this amendment. The §3 sentence that already permits upstreaming "as a normal GitHub PR through argoproj's contribution flow" is now the canonical operating mode, expanded by the `UPSTREAMING.md` tracking discipline.
- The "no upstream feedback loop" item in the original *Consequences → Negative* list is downgraded: the feedback loop now exists (via `UPSTREAMING.md`), and the residual risk is that Skaphos forgets to *use* it. The author checklist is the mitigation.
- `docs/DECISIONS.md` is updated to flag this amendment alongside the ADR 0007 row.

## Context

ADR 0006 went through two same-day amendments to land at "consume vanilla upstream pseudo-version, accept the k8s.io ≤ v0.34 ceiling as a steady-state constraint." The afternoon amendment closed the soft-fork (`skaphos/argo-cd` mirror + replace directive + upstream PR) because the v2beta scheme issue isn't a 50-LOC dead-import cleanup — `gitops-engine/pkg/utils/kube/scheme/scheme.go` blanket-registers Kubernetes API groups via `_ "k8s.io/kubernetes/pkg/apis/autoscaling/install"`, which transitively pulls in `autoscaling/v2beta1` and `v2beta2` types that were removed from `k8s.io/api` at v0.35. The scheme-install path is reached through `pkg/sync` cluster cache initialization, so any non-trivial use of the engine traverses it.

That afternoon's framing left Keleustes pinned to k8s.io ≤ v0.34 / controller-runtime ≤ v0.22 indefinitely, with the ceiling lifting only if upstream restructured scheme registration on its own initiative. The maintenance-posture signals catalogued in the SKA-327 spike report (no SemVer tag since v0.7.3 in 2022; issues route through argo-cd's tracker where they compete with Argo CD's product roadmap; the dead v2beta imports have survived multiple Argo CD minor releases) make that lift unlikely on a near-term cadence.

The cost of *not* lifting the ceiling compounds. govulncheck flags two CVEs in the k8s.io v0.34 baseline that we currently carry as advisory-only (PR #6 ci(github-actions) leaves the vuln job non-blocking for exactly this reason). Kubebuilder, controller-runtime, and the broader operator-SDK ecosystem move at k8s minor cadence; staying at v0.34 means Keleustes drifts further from upstream Go module conventions every quarter. By MVP 3 the divergence will be load-bearing.

## Decision

Skaphos hard-forks `gitops-engine` out of `argo-cd/gitops-engine/` into a standalone repository at `github.com/skaphos/gitops-engine`. Keleustes' `go.mod` requires the fork directly. The fork is the canonical upstream for every Skaphos product that needs sync, diff, health, or cluster-cache primitives from now forward.

### 1. Repository structure

- **Source of the extraction.** `git filter-repo --subdirectory-filter gitops-engine/` against the current `skaphos-patches` branch of the existing `../argo-cd` clone. The branch already sits on top of the upstream commit `a39953d` that Keleustes' vanilla pseudo-version pins (`v0.0.0-20260515214037-a39953d21f51`); the one patch on top (`04a7370 chore(gitops-engine): drop dead autoscaling/v2beta1 and v2beta2 references`) is preserved by filter-repo as the first Skaphos delta.
- **Destination.** `github.com/skaphos/gitops-engine`. The existing `skaphos/argo-cd` fork has been renamed in place to claim that name; GitHub's redirect preserves the old URL for existing consumers (notably the local `../argo-cd` working directory's `origin` remote) for the standard redirect window.
- **Module path.** `github.com/skaphos/gitops-engine`. Renaming the Go module path is committed — no path-compatibility obligation exists since `github.com/argoproj/gitops-engine` was already archived in 2025-09 and redirected into argo-cd. A single commit on top of the filter-repo'd history performs the `go mod edit -module` plus internal import rewrite; that commit is the boundary between "byte-equivalent to argo-cd's gitops-engine/ at a39953d" and "Skaphos-owned codebase."

### 2. License and attribution

`gitops-engine` is Apache-2.0 (inherited from Argo CD). Derivative works must remain Apache-2.0; relicensing requires copyright-holder consent that we neither have nor will pursue.

- `LICENSE` in the fork is the verbatim Apache-2.0 text copied from `argo-cd/LICENSE`.
- `NOTICE` carries upstream's content verbatim (the Argo CD Project and contributors, CNCF) with an appended modifications stanza in the form recommended by Apache-2.0 §4(d): a separate paragraph identifying the modifier ("Modifications copyright 2026 Skaphos and contributors. Modifications licensed under the Apache License, Version 2.0."), placed below the upstream NOTICE text without altering it. The stanza is concise on purpose — NOTICE files are durable, and verbose attribution becomes a maintenance liability.
- Keleustes' existing `THIRD_PARTY_LICENSES.md` aggregation (SKA-420) picks up the new module path automatically on the next `task licenses` run; no infrastructure change.

### 3. Backport workflow

The fork tracks `argoproj/argo-cd` as `upstream-monorepo` and pulls in changes via `git format-patch --relative=gitops-engine/` against the subdirectory, then `git am` into the fork. The `--relative` flag handles the path translation between `gitops-engine/foo.go` upstream and `foo.go` in the fork. Patches that conflict with Skaphos changes are resolved patch-by-patch; deliberately skipped patches are logged with one-line rationale in `UPSTREAM_SYNC.md` so the audit trail survives.

Cadence: every argo-cd minor release **or** quarterly, whichever fires first. Critical CVEs that land upstream trigger out-of-band syncs scoped to the specific affected commits.

The fork does **not** ship patches back to argoproj. If a Skaphos change is genuinely upstreamable — e.g., the scheme-registration refactor — it goes upstream as a normal GitHub PR through argoproj's contribution flow, not through the format-patch series. The series exists to record Skaphos divergence, not to bridge contribution back upstream.

> **Superseded by the [2026-05-18 amendment](#2026-05-18--friendly-fork-posture-clarification) above.** The fork is now a *friendly fork* that actively maintains the intent to upstream. Skaphos changes are shaped so they can be PR'd to argoproj verbatim, and the [`UPSTREAMING.md`](https://github.com/skaphos/gitops-engine/blob/main/UPSTREAMING.md) on the fork repo tracks every outbound candidate. The "format-patch series records divergence" framing stands — `UPSTREAMING.md` is the new bridge.

### 4. Containment rule and engine boundaries (unchanged)

ADR 0006 §4's containment rule continues to apply: `gitops-engine` imports live only under `internal/{sync,diff,health,kube}/` in Keleustes. The fork doesn't change Keleustes' package boundaries, engine ownership, render policy, Git-provider policy, or annotation policy. It changes *where the module lives* and *who owns its release cadence* — nothing else.

The mandatory `replace` block in Keleustes' `go.mod` (ADR 0006 amendment §3) remains required: the fork inherits the upstream `require k8s.io/* v0.0.0` + `replace` pattern, and Go's module system still doesn't propagate replace directives from dependencies. The block stays until the scheme refactor lands; see §5 below.

### 5. k8s.io ceiling — still pinned, but now liftable

Owning the fork does not by itself lift the k8s.io ≤ v0.34 ceiling. The structural cause (`pkg/utils/kube/scheme/scheme.go` blanket-registering `autoscaling/install`) is unchanged by the rehoming. What changes is that the work to fix it is now a Skaphos-internal commit on the fork, not an upstream argoproj contribution gated on Akuity's review cadence.

The ceiling lift lands as a dedicated Linear ticket (the existing SKA-421 "gitops-engine k8s.io v0.35+ enablement: scheme modernization + parser.go data cleanup" is rescoped to target the fork rather than upstream). When that ticket lands, Keleustes' `go.mod` drops the v0.34 `replace` block and pins forward; govulncheck's two advisory findings clear in the same change.

## Consequences

### Positive

- **Decoupled release cadence.** Skaphos can ship k8s.io ceiling lifts, security fixes, and operator-facing features on its own schedule. No external blocker on MVP 3 timing.
- **Cleaner module hygiene.** Renaming to `github.com/skaphos/gitops-engine` collapses the conceptual oddity of importing a Go module that lives inside another project's monorepo. Go-aware tooling (gopls, dependency dashboards, SBOM emitters) treats the fork as a first-class module.
- **govulncheck path-to-green.** Once SKA-421 lands against the fork, the k8s.io v0.34 baseline CVEs clear. CI's vuln job can move from `continue-on-error: true` to required.
- **Patch-series audit trail.** The format-patch + `am` workflow produces a kernel-style numbered series of every Skaphos delta. Re-exportable at any time; useful for security review, license audits, and future upstreaming.
- **Existing Argo CD ecosystem still usable.** Other Argo CD components (`applicationset`, `notifications-engine`, the CLI) remain consumable by Keleustes operators side-by-side; the fork covers only `gitops-engine` and doesn't claim to be a full Argo CD replacement.

### Negative

- **Permanent maintenance cost.** Skaphos now owns a sync/diff/health/cache library it didn't write. Every k8s minor bump, every dependency update, every security patch is a Skaphos engineering task. Realistic budget: 1–2 engineer-weeks per quarter once the backport workflow is fluent.
- **Divergence risk.** As Skaphos and upstream evolve independently, the conflict surface on each sync grows. Some upstream commits will be deliberately skipped, and that decision compounds over time. Mitigated by the per-skip rationale log in `UPSTREAM_SYNC.md` but not eliminated.
- **No upstream feedback loop.** Skaphos changes that would benefit the broader Argo CD ecosystem won't reach it unless explicitly upstreamed as separate PRs — easy to forget when the in-tree path works.
- **Brand-attribution care.** The fork is clearly derived from Argo CD; documentation must say so plainly without overclaiming Skaphos's contribution. Risk surfaces if marketing or docs describe the fork as a Skaphos-originated library.

### Neutral

- **GitHub fork relationship.** GitHub will continue to display `skaphos/gitops-engine` as a fork of `argoproj/argo-cd` in the UI even after we force-push the filter-repo'd history. This is cosmetic and doesn't affect functionality; detaching requires a GitHub support request and isn't worth pursuing.
- **`mfacenet/argo-cd` remote.** The existing `../argo-cd` clone carries a `mfacenet` remote that is unrelated to this decision. It can be removed or kept as the operator-facing fork sees fit; no Keleustes implication.

## Compliance and follow-ups

1. **Drop a supersession marker on ADR 0006.** The afternoon-amendment's *Decision* paragraph (the one that freezes the v0.34 ceiling as steady-state) gets a `> **Superseded by [ADR 0007](./0007-hard-fork-gitops-engine.md).** <one-line note>` blockquote. Same pattern for §4's "vanilla upstream" implication.
2. **Update [`docs/DECISIONS.md`](../DECISIONS.md)** — add the ADR 0007 row and update ADR 0006's row note to reflect that its later amendments are partially superseded.
3. **File the extraction Linear ticket.** New SKA-### "Extract gitops-engine into skaphos/gitops-engine repo" referencing this ADR. Sub-tasks: filter-repo extraction, module rename commit, push to renamed GitHub repo, baseline tag, `UPSTREAM_SYNC.md` template, CI workflow, README.
4. **Rescope SKA-421.** The "k8s.io v0.35+ enablement" ticket targets the fork's scheme-registration code path rather than upstream argoproj.
5. **Keleustes `go.mod` swap.** Once the fork repo is live and tagged, a Keleustes PR replaces `require github.com/argoproj/argo-cd/gitops-engine v0.0.0-...` with `require github.com/skaphos/gitops-engine vX.Y.Z`. The `replace` block stays until SKA-421 lands.
6. **`docs/plans/2026-05-gitops-engine-spike.md`** — add an annotation referencing this ADR, similar to how ADR 0006's amendments were marked.
