// SPDX-FileCopyrightText: 2026 Rillan AI LLC
// SPDX-License-Identifier: MIT
import type { ReactNode } from 'react'

export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string
  description?: string
  actions?: ReactNode
}) {
  return (
    <div className="mb-6 flex items-start justify-between gap-4">
      <div>
        <h1 className="text-xl font-semibold">{title}</h1>
        {description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  )
}

/**
 * Placeholder body for a stubbed screen. Each links back to the design-spec
 * section so the high-fidelity pass knows what belongs here.
 */
export function ScreenStub({ specRef, children }: { specRef: string; children?: ReactNode }) {
  return (
    <div className="rounded-lg border border-dashed p-8 text-sm text-muted-foreground">
      <p className="font-medium text-foreground">Scaffolded screen — pending design.</p>
      <p className="mt-1">
        See <code className="rounded bg-muted px-1 py-0.5">{specRef}</code> in{' '}
        <code className="rounded bg-muted px-1 py-0.5">docs/design/ui-design-spec.md</code>.
      </p>
      {children && <div className="mt-4">{children}</div>}
    </div>
  )
}
