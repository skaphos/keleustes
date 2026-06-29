// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router-dom'
import { AuthProvider, getActiveToken } from '@/auth/auth'
import { setAuthToken } from '@/api/client'
import { router } from '@/router'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, retry: 1 } },
})

// Carry the current OIDC token on every API request (ADR 0004). Reads live auth
// state, so sign-out drops the token rather than the client pinning a constant.
setAuthToken(getActiveToken)

async function enableMocking() {
  // MSW serves the contract fixtures in dev so the shell is fully navigable
  // before the Go API server exists. Disable by setting VITE_USE_MSW=false.
  if (import.meta.env.PROD || import.meta.env.VITE_USE_MSW === 'false') return
  const { worker } = await import('@/mocks/browser')
  await worker.start({ onUnhandledRequest: 'bypass' })
}

function renderApp() {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <RouterProvider router={router} />
        </AuthProvider>
      </QueryClientProvider>
    </StrictMode>,
  )
}

// Always mount the app — if MSW fails to start (e.g. the worker file is
// missing), log it and render anyway rather than leaving a blank page.
enableMocking()
  .catch((err) => console.warn('[msw] mock backend failed to start; continuing', err))
  .finally(renderApp)
