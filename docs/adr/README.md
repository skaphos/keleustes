<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Architecture Decision Records

ADRs in this directory capture architecturally significant decisions for
Keleustes. New ADRs follow the format described by the `adr-write` skill in
the team's shared tooling. Each ADR is immutable once accepted; supersede
rather than edit.

The **[Architecture Decisions Living Index](../DECISIONS.md)** is the
single source of truth for "what we have actually decided" — it lists
every accepted ADR plus any deep-dive plans that have stabilized into
active interim contracts. Start there when in doubt about whether a
given assumption is current.

## When an ADR supersedes earlier text

ADRs supersede individual sections of `docs/PROPOSAL.md` and earlier
deep-dive plans in `docs/plans/` as design intent becomes
implementation. To keep the three layers from silently drifting:

1. **Write the ADR.** Use the `adr-write` skill; cite the specific
   PROPOSAL section(s) or plan section(s) it supersedes in the
   `Supersedes:` front-matter line.
2. **Open a companion PR** (small, separate from the ADR PR when
   convenient) that touches each superseded passage with a single
   blockquote marker:

   ```markdown
   > **Superseded by [ADR 00XX](../adr/00XX-short-name.md).** <one-sentence note on what changed.>
   ```

   The original passage stays put. The marker is what prevents the
   "we all know that changed" tax.
3. **Update [`docs/DECISIONS.md`](../DECISIONS.md)** with the new
   row (or update the existing row's status if the ADR amends an
   earlier one). The DECISIONS index is the only place where ADRs,
   interim contracts, and supersession pointers all appear together.

For active interim contracts (deep-dive plans that have stabilized
but not yet been promoted into an ADR), follow the same pattern with
`> **Superseded by [docs/plans/YYYY-MM-name.md](../plans/YYYY-MM-name.md)
(SKA-XXX, active interim contract)** — promotes to an ADR.` as the
marker text.

## Authoring conventions

- File name: `NNNN-short-kebab-case.md`. NNNN is the next available
  four-digit number, zero-padded.
- Front matter: `Status`, `Date`, `Deciders`, `Linear`, `Related`,
  `Supersedes` (when applicable).
- One decision per ADR. If a single change covers two genuinely
  independent decisions, write two ADRs.
- ADRs are immutable once accepted. Corrections that change the
  decision land as a new ADR that supersedes the old one;
  corrections that clarify intent without changing the decision
  land as a dated amendment section appended to the ADR.
