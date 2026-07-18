// SPDX-FileCopyrightText: 2026 Rillan AI LLC
// SPDX-License-Identifier: MIT
import { PageHeader, ScreenStub } from '@/components/PageHeader'

export function Environments() {
  return (
    <div>
      <PageHeader
        title="Environments"
        description="Topology: environment → cell → deployment target, with health and freeze windows."
      />
      <ScreenStub specRef="§6.8 Environments / topology" />
    </div>
  )
}
