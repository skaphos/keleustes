// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { setupWorker } from 'msw/browser'
import { handlers } from './handlers'

/** Dev-only Service Worker that serves the contract fixtures (see main.tsx). */
export const worker = setupWorker(...handlers)
