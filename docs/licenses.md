<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# License attribution

Keleustes is distributed under the MIT License (see `LICENSE` at the
repo root). The compiled artifacts (`manager`, `keleustesctl`, and the
future `agent`) embed third-party Go modules under a mix of permissive
licenses — Apache-2.0, MIT, BSD-3-Clause, ISC, MPL-2.0. The two files
at the repo root that carry the attribution this requires:

- **`THIRD_PARTY_LICENSES.md`** — generated tabulation of every Go
  package reachable from `./cmd/manager` and `./cmd/keleustesctl`,
  listing module path, SPDX license identifier, and source URL.
- **`NOTICE`** — Keleustes' own copyright statement, a pointer back to
  the tabulation, and (as adopted) per-module NOTICE excerpts for
  Apache-2.0 dependencies that ship their own NOTICE files per
  Apache-2.0 §4(d).

This setup satisfies the license-attribution clause of
[ADR 0006 §4](adr/0006-engine-boundaries.md) and lays the groundwork
for adopting future Apache-2.0 dependencies without re-litigating the
contract each time.

## Regenerate after dep changes

Any `go.mod` change that adds or upgrades a module invalidates
`THIRD_PARTY_LICENSES.md`. Workflow:

1. Make the dependency change.
2. Run `go -C tools tool task licenses`.
3. Review the diff (often the only change is a version bump in a URL).
4. `git add THIRD_PARTY_LICENSES.md` and commit alongside the
   dependency change.

CI runs `go -C tools tool task licenses:check`, which regenerates and
fails on any drift. The failure message is also the fix instruction.

## Adopting an Apache-2.0 dep with its own NOTICE

Most Apache-2.0 deps don't ship a `NOTICE` file. When one does
(Apache-2.0 §4(d) is mandatory in that case), append the upstream
NOTICE content to `NOTICE` under a heading like
`## Notice from <module>`. This is manual and rare; document the
adoption in the PR description so it is not forgotten on the next
rebase.

## Why generate rather than scan at build time

`THIRD_PARTY_LICENSES.md` is committed because:

- The PR review needs to surface license changes as part of the diff.
- Release artifacts are reproducible without internet access to the
  Go module proxy.
- Customers asking for the license bill of materials can read the
  current file from `main` directly.

## Implementation

The generation script is `hack/licenses/generate.sh`. It calls
[`google/go-licenses`](https://github.com/google/go-licenses) (pinned
in the task command) against the manager and ctl binaries, sorts the
CSV output for stable diffs, and renders it as a Markdown table inside
a fixed front-matter / footer.

Future expansion when the agent binary lands: add `./cmd/agent` to the
`TARGETS` array in `hack/licenses/generate.sh`.
