<!--
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
-->

# Keleustes UI

The Keleustes GitOps control-plane web UI. **Scaffold stage** — an app shell
with the ~10 screens from [`docs/design/ui-design-spec.md`](../docs/design/ui-design-spec.md)
stubbed, wired to a contract-typed mock backend so the UI is fully navigable and
testable before the real API server exists.

## Stack

| Concern | Choice |
|---|---|
| Build / dev | Vite + React 19 + TypeScript |
| Styling | Tailwind CSS + shadcn/ui (Radix); components owned in `src/components/ui` |
| Routing | React Router |
| Server state | TanStack Query |
| API client | `openapi-fetch`, typed from [`openapi/keleustes.v1.yaml`](../openapi/keleustes.v1.yaml) |
| Mock backend | MSW (Mock Service Worker) + fixtures in `src/mocks` |
| Unit / component tests | Vitest + React Testing Library |
| E2E | Playwright |

## Backend strategy (staged)

The UI talks to `/api/v1` (the contract). Today those requests are served by
**MSW fixtures** (`src/mocks`) so you can click around and we can test. When the
**Go API server** lands (`internal/api`, phase 2), it implements the **same**
`openapi/keleustes.v1.yaml`; the UI just stops mocking (`VITE_USE_MSW=false`) and
points at the real server. The contract is the constant.

## Commands

Run via Task from the repo root (preferred):

```
go -C tools tool task ui:install   # pnpm install
go -C tools tool task ui:dev        # Vite dev server + MSW (usability)
go -C tools tool task ui:gen        # regenerate the typed API client from openapi/
go -C tools tool task ui:test       # Vitest (unit + component)
go -C tools tool task ui:lint       # eslint + typecheck
go -C tools tool task ui:build      # production build
go -C tools tool task ui:e2e        # Playwright smoke (boots dev server)
```

Or directly in `ui/` with `pnpm <script>` (see `package.json`).

## Layout

```
ui/
  ../openapi/keleustes.v1.yaml   # API contract (shared with CLI + future Go server)
  src/
    api/        # typed client (schema.d.ts generated from the contract)
    auth/       # OIDC token plumbing (STUB; real impl = SKA-330)
    components/ # shell (nav rail, context bar) + ui/ primitives + StatusBadge
    lib/        # cn(), status vocabulary (§3.1)
    mocks/      # MSW worker/server + fixtures
    routes/     # the ~10 screens (Applications + Audit + Promotions are data-backed)
    router.tsx  # routes (§4)
  e2e/          # Playwright specs
```

## Notes

- **First run:** `pnpm dlx msw init public/` writes the Service Worker MSW needs
  in the browser (gitignored). `ui:dev` reminds you if it's missing.
- The typed client `src/api/schema.d.ts` is generated (`ui:gen`) and gitignored;
  CI regenerates it.
- Hard UI constraints (read + three write actions, no inline edits, ULID
  deep-links) are documented in the design spec and must hold.
