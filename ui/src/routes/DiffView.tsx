// SPDX-FileCopyrightText: 2026 Rillan AI LLC
// SPDX-License-Identifier: MIT
import { PageHeader, ScreenStub } from '@/components/PageHeader'

export function DiffView() {
  return (
    <div>
      <PageHeader
        title="Diff"
        description="Git ↔ live, release ↔ release, env ↔ env, rendered manifest, policy."
      />
      <ScreenStub specRef="§6.5 Diff view">
        Read-only. Drift resolves via Promote (reconcile through Git) — never "make live match".
      </ScreenStub>
    </div>
  )
}
