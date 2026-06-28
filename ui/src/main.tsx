// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router-dom'
import { AuthProvider } from '@/auth/auth'
import { setAuthToken } from '@/api/client'
import { router } from '@/router'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, retry: 1 } },
})

// Carry the (stub) OIDC token on every API request (ADR 0004).
setAuthToken(() => 'stub-token')

async function enableMocking() {
  // MSW serves the contract fixtures in dev so the shell is fully navigable
  // before the Go API server exists. Disable by setting VITE_USE_MSW=false.
  if (import.meta.env.PROD || import.meta.env.VITE_USE_MSW === 'false') return
  const { worker } = await import('@/mocks/browser')
  await worker.start({ onUnhandledRequest: 'bypass' })
}

enableMocking().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <RouterProvider router={router} />
        </AuthProvider>
      </QueryClientProvider>
    </StrictMode>,
  )
})
