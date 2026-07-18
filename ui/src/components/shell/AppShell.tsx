// SPDX-FileCopyrightText: 2026 Rillan AI LLC
// SPDX-License-Identifier: MIT
import { Outlet } from 'react-router-dom'
import { NavRail } from './NavRail'
import { ContextBar } from './ContextBar'

/** App shell: persistent nav rail + context bar + routed content (§4). */
export function AppShell() {
  return (
    <div className="flex h-screen overflow-hidden">
      <NavRail />
      <div className="flex min-w-0 flex-1 flex-col">
        <ContextBar />
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
