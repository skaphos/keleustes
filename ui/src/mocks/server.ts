// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { setupServer } from 'msw/node'
import { handlers } from './handlers'

/** Node MSW server for the Vitest suite (wired in src/test/setup.ts). */
export const server = setupServer(...handlers)
