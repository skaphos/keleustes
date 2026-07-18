// SPDX-FileCopyrightText: 2026 Rillan AI LLC
// SPDX-License-Identifier: MIT
//
// Canonical Keleustes status vocabulary. Mirror of the `Status` enum in
// openapi/keleustes.v1.yaml and §3.1 of docs/design/ui-design-spec.md. Keep
// these in sync: one label + color + glyph per state, used everywhere.

export type Status =
  | 'Healthy'
  | 'Progressing'
  | 'Degraded'
  | 'Drifted'
  | 'Blocked'
  | 'Frozen'
  | 'Missing'
  | 'Failed'

export interface StatusMeta {
  label: string
  /** Tailwind text color token (see the `@theme` block in src/index.css). */
  color: string
  /** Single-glyph affordance for dense tables/matrix cells. */
  glyph: string
}

export const STATUS_META: Record<Status, StatusMeta> = {
  Healthy: { label: 'Healthy', color: 'text-status-healthy', glyph: '●' },
  Progressing: { label: 'Progressing', color: 'text-status-progressing', glyph: '◐' },
  Degraded: { label: 'Degraded', color: 'text-status-degraded', glyph: '⚠' },
  Drifted: { label: 'Drifted', color: 'text-status-drifted', glyph: '⤳' },
  Blocked: { label: 'Blocked', color: 'text-status-blocked', glyph: '⛔' },
  Frozen: { label: 'Frozen', color: 'text-status-frozen', glyph: '❄' },
  Missing: { label: 'Missing', color: 'text-status-missing', glyph: '◌' },
  Failed: { label: 'Failed', color: 'text-status-failed', glyph: '✗' },
}

export function statusMeta(status: Status): StatusMeta {
  return STATUS_META[status] ?? STATUS_META.Missing
}
