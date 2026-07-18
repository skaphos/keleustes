// SPDX-FileCopyrightText: 2026 Rillan AI LLC
// SPDX-License-Identifier: MIT
import { Badge } from '@/components/ui/badge'
import { statusMeta, type Status } from '@/lib/status'
import { cn } from '@/lib/utils'

/** Renders a status with the canonical glyph + color (ui-design-spec §3.1). */
export function StatusBadge({ status, className }: { status: Status; className?: string }) {
  const meta = statusMeta(status)
  return (
    <Badge variant="muted" className={cn('font-normal', className)} aria-label={meta.label}>
      <span className={meta.color} aria-hidden>
        {meta.glyph}
      </span>
      {meta.label}
    </Badge>
  )
}
