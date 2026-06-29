// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { describe, expect, it } from 'vitest'
import { STATUS_META, statusMeta, type Status } from './status'

describe('status vocabulary', () => {
  it('has meta for every canonical status', () => {
    const statuses: Status[] = [
      'Healthy',
      'Progressing',
      'Degraded',
      'Drifted',
      'Blocked',
      'Frozen',
      'Missing',
      'Failed',
    ]
    for (const s of statuses) {
      expect(STATUS_META[s]).toBeDefined()
      expect(STATUS_META[s].color).toMatch(/^text-status-/)
    }
  })

  it('falls back to Missing for an unknown status', () => {
    expect(statusMeta('Nope' as Status)).toBe(STATUS_META.Missing)
  })
})
