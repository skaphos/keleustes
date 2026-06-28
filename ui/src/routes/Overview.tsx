// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { PageHeader, ScreenStub } from '@/components/PageHeader'

export function Overview() {
  return (
    <div>
      <PageHeader title="Overview" description="Fleet health at a glance." />
      <ScreenStub specRef="§6.1 Overview">
        Tiles: fleet health rollup · approvals assigned to me · active promotions · recent activity.
      </ScreenStub>
    </div>
  )
}
