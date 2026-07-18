// SPDX-FileCopyrightText: 2026 Rillan AI LLC
// SPDX-License-Identifier: MIT
import { NavLink } from 'react-router-dom'
import { NAV_ITEMS } from './nav'
import { useAuth } from '@/auth/auth'
import { cn } from '@/lib/utils'

export function NavRail() {
  const { can } = useAuth()
  return (
    <nav
      aria-label="Primary"
      className="flex h-full w-56 shrink-0 flex-col gap-1 border-r bg-card px-3 py-4"
    >
      <div className="px-2 pb-4 text-sm font-semibold tracking-wide text-foreground">
        KELEUSTES
      </div>
      {NAV_ITEMS.filter((item) => !item.adminOnly || can('admin')).map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.to === '/'}
          className={({ isActive }) =>
            cn(
              'flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
              isActive
                ? 'bg-accent text-accent-foreground'
                : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
            )
          }
        >
          <item.icon className="h-4 w-4" aria-hidden />
          {item.label}
        </NavLink>
      ))}
    </nav>
  )
}
