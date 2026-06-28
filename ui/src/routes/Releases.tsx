// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { PageHeader, ScreenStub } from '@/components/PageHeader'

export function Releases() {
  return (
    <div>
      <PageHeader
        title="Releases"
        description="Deployable artifacts and their provenance (signature, SBOM, source commit)."
      />
      <ScreenStub specRef="§6.6 Releases" />
    </div>
  )
}
