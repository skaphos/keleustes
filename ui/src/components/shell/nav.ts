// SPDX-FileCopyrightText: 2026 Rillan AI LLC
// SPDX-License-Identifier: MIT
import {
  LayoutDashboard,
  Grid3x3,
  GitPullRequestArrow,
  Package,
  Network,
  ScrollText,
  Settings,
  type LucideIcon,
} from 'lucide-react'

export interface NavItem {
  to: string
  label: string
  icon: LucideIcon
  /** Verb required to see this section (server-enforced; UI hides if absent). */
  verb?: string
  /** Restrict to admins. */
  adminOnly?: boolean
}

// Information architecture — ui-design-spec §4.
export const NAV_ITEMS: NavItem[] = [
  { to: '/', label: 'Overview', icon: LayoutDashboard },
  { to: '/applications', label: 'Applications', icon: Grid3x3 },
  { to: '/promotions', label: 'Promotions', icon: GitPullRequestArrow },
  { to: '/releases', label: 'Releases', icon: Package },
  { to: '/environments', label: 'Environments', icon: Network },
  { to: '/audit', label: 'Audit', icon: ScrollText },
  { to: '/admin', label: 'Admin', icon: Settings, adminOnly: true },
]
