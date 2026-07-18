// SPDX-FileCopyrightText: 2026 Rillan AI LLC
// SPDX-License-Identifier: MIT
import { setupWorker } from 'msw/browser'
import { handlers } from './handlers'

/** Dev-only Service Worker that serves the contract fixtures (see main.tsx). */
export const worker = setupWorker(...handlers)
