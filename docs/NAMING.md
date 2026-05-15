<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Naming

## Identity

- Tool name: **Keleustes**
- Former working name: **Pilot** (retired due to Kubernetes-ecosystem collision)
- Naming status: settled, collision-reviewed 2026-05-14
- Repository: `skaphos/keleustes`
- System: Skaphos
- Primary module: `github.com/skaphos/keleustes`
- Primary language: Go
- Frontend language: TypeScript

## Rationale

`Keleustes` (Greek κελευστής) was the officer on a trireme who set the stroke
and cadence for the rowers — coordinating movement and direction through
constrained passage. The metaphor maps to GitOps delivery: the system directs
release cadence across environments, it does not power the change itself. The
name also reinforces the Skaphos Greek-nautical register (Skaphos itself is
Greek for *vessel/hull*).

Pronunciation: `kuh-LOO-stees` (anglicized); Greek `keh-loo-STEES`.

## Collision review (2026-05-14)

- `github.com/keleustes` is a dormant GitHub org (last activity ~2019–2023)
  holding Kubernetes-adjacent forks (a kustomize fork, armada-operator,
  cluster-api-provider-airship, capi-yaml-gen). No active project, no published
  packages on any active registry. Accepted as **search-noise only**, not a
  semantic collision.
- `github.com/retr0h/keleustes` is an abandoned 2017 xhyve VM tool; unrelated.
- CNCF landscape, Artifact Hub, npm, crates.io, PyPI, and active `pkg.go.dev`
  modules: no collisions.
- No company, product, or trademark registered under Keleustes or the Latin
  transliteration Celeustes.
- Domains `.io`, `.dev`, `.sh`, `.org` returned as available.

Skaphos disambiguates by publishing under the `skaphos/keleustes` repository
path. The Latin transliteration `Celeustes` is held as a fallback if the
dormant `github.com/keleustes` org is reactivated by a third party.
