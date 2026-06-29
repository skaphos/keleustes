// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { PageHeader, ScreenStub } from '@/components/PageHeader'

export function Admin() {
  return (
    <div>
      <PageHeader
        title="Admin"
        description="Topology · policy · RBAC · identity. Changes flow through Git/CRD PRs."
      />
      <ScreenStub specRef="§6.9 Admin" />
    </div>
  )
}
