// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Disable the MSW mock backend (point at a real API server). */
  readonly VITE_USE_MSW?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
