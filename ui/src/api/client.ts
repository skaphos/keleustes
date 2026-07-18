// SPDX-FileCopyrightText: 2026 Rillan AI LLC
// SPDX-License-Identifier: MIT
//
// Contract-typed REST client. Types come from openapi/keleustes.v1.yaml via
// `pnpm gen:api` (openapi-typescript → ./schema.d.ts). In dev and test the
// requests are intercepted by MSW (src/mocks); in production this base URL is
// served by the Go API server implementing the same contract.
import createClient from 'openapi-fetch'
import type { paths } from './schema'

// Same-origin API. Resolve against the page origin so the base URL is absolute
// — browsers accept a relative base, but the Node/undici fetch used in the test
// runtime requires an absolute URL. MSW matches on the path either way.
const ORIGIN =
  typeof window !== 'undefined' && window.location?.origin
    ? window.location.origin
    : 'http://localhost'

export const API_BASE = `${ORIGIN}/api/v1`

/**
 * The shared API client. A request interceptor attaches the OIDC bearer token
 * (ADR 0004 — identity is OIDC, authz is server-enforced; the UI only carries
 * the token, it never decides permissions).
 */
export const api = createClient<paths>({
  baseUrl: API_BASE,
  // Resolve the global fetch lazily on each call rather than capturing it at
  // client-creation time. Without this, the client would bind the unpatched
  // fetch before MSW installs its interceptor in tests (and any other runtime
  // that swaps fetch after module load).
  fetch: (request) => globalThis.fetch(request),
})

export function setAuthToken(getToken: () => string | null): void {
  api.use({
    onRequest({ request }) {
      const token = getToken()
      if (token) request.headers.set('Authorization', `Bearer ${token}`)
      return request
    },
  })
}
