// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
//
// OIDC auth plumbing — STUB. ADR 0004: identity is OIDC, authorization is
// verb-scoped and enforced by the API server. The UI only (a) carries the
// token and (b) asks the server what the user may do, rendering actions
// accordingly. It never enforces permissions itself.
//
// This stub provides a fake identity + a permissive `can()` so the shell is
// navigable offline. Real OIDC (PKCE redirect to the IdentityProvider, token
// refresh, /whoami + verb set) lands with SKA-330.
import { createContext, use, useMemo, useState, type ReactNode } from 'react'

export interface Identity {
  subject: string
  name: string
  email: string
  idp: string
}

export interface AuthState {
  identity: Identity | null
  token: string | null
  /** Verb check — server-truth in production; permissive in the stub. */
  can: (verb: string, resource?: string) => boolean
  signOut: () => void
}

const STUB_IDENTITY: Identity = {
  subject: 'u_stub',
  name: 'Dev Operator',
  email: 'operator@keleustes.local',
  idp: 'stub-oidc',
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [identity, setIdentity] = useState<Identity | null>(STUB_IDENTITY)

  const value = useMemo<AuthState>(
    () => ({
      identity,
      token: identity ? 'stub-token' : null,
      // Stub: allow everything. Real impl queries the server's verb set.
      can: () => true,
      signOut: () => setIdentity(null),
    }),
    [identity],
  )

  return <AuthContext value={value}>{children}</AuthContext>
}

export function useAuth(): AuthState {
  const ctx = use(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within <AuthProvider>')
  return ctx
}
