// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { Search } from 'lucide-react'
import { useAuth } from '@/auth/auth'
import { Button } from '@/components/ui/button'

/**
 * Global context bar — scope selectors + search + identity (ui-design-spec §4).
 * Selectors are presentational stubs; wiring lands with the screens that
 * consume the scope.
 */
export function ContextBar() {
  const { identity, signOut } = useAuth()
  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b bg-card px-4">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <select className="rounded-md border bg-background px-2 py-1" aria-label="Project scope">
          <option>All projects</option>
        </select>
        <select className="rounded-md border bg-background px-2 py-1" aria-label="Environment">
          <option>All environments</option>
        </select>
        <select className="rounded-md border bg-background px-2 py-1" aria-label="Region">
          <option>All regions</option>
        </select>
      </div>
      <div className="relative ml-2 flex-1 max-w-md">
        <Search className="pointer-events-none absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <input
          className="w-full rounded-md border bg-background py-1.5 pl-8 pr-3 text-sm"
          placeholder="Search…  (⌘K)"
          aria-label="Search"
        />
      </div>
      <div className="ml-auto flex items-center gap-3">
        <div className="text-right text-xs leading-tight">
          <div className="font-medium">{identity?.name ?? 'Signed out'}</div>
          <div className="text-muted-foreground">{identity?.idp}</div>
        </div>
        <Button variant="outline" size="sm" onClick={signOut}>
          Sign out
        </Button>
      </div>
    </header>
  )
}
