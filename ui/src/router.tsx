// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { createBrowserRouter } from 'react-router-dom'
import { AppShell } from '@/components/shell/AppShell'
import { Overview } from '@/routes/Overview'
import { Applications } from '@/routes/Applications'
import { ApplicationDetail } from '@/routes/ApplicationDetail'
import { DiffView } from '@/routes/DiffView'
import { Promotions } from '@/routes/Promotions'
import { PromotionDetail } from '@/routes/PromotionDetail'
import { Releases } from '@/routes/Releases'
import { Environments } from '@/routes/Environments'
import { Audit } from '@/routes/Audit'
import { Admin } from '@/routes/Admin'
import { NotFound } from '@/routes/NotFound'

// Routes — ui-design-spec §4. ULIDs in detail paths (stable across renames).
export const router = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <Overview /> },
      { path: 'applications', element: <Applications /> },
      { path: 'applications/:appUlid', element: <ApplicationDetail /> },
      { path: 'applications/:appUlid/diff', element: <DiffView /> },
      { path: 'promotions', element: <Promotions /> },
      { path: 'promotions/:promotionUlid', element: <PromotionDetail /> },
      { path: 'releases', element: <Releases /> },
      { path: 'environments', element: <Environments /> },
      { path: 'audit', element: <Audit /> },
      { path: 'admin', element: <Admin /> },
      { path: '*', element: <NotFound /> },
    ],
  },
])
