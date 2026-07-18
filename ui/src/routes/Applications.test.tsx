// SPDX-FileCopyrightText: 2026 Rillan AI LLC
// SPDX-License-Identifier: MIT
import { describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import { renderWithProviders } from '@/test/utils'
import { Applications } from './Applications'

// Exercises the full contract → openapi-fetch → Query → MSW pipeline: the
// fixtures in src/mocks satisfy openapi/keleustes.v1.yaml, so this asserts on
// real data shapes, not hand-rolled stubs.
describe('Applications matrix', () => {
  it('renders a row per application with deployed versions', async () => {
    renderWithProviders(<Applications />)

    await waitFor(() => expect(screen.getByText('api')).toBeInTheDocument())
    expect(screen.getByText('web')).toBeInTheDocument()
    expect(screen.getByText('worker')).toBeInTheDocument()

    // Versions from the fixtures render in cells.
    expect(screen.getAllByText('1.4.2').length).toBeGreaterThan(0)
    // Column headers from the matrix contract.
    expect(screen.getByText('prod-eu')).toBeInTheDocument()
  })

  it('shows the snapshot freshness indicator', async () => {
    renderWithProviders(<Applications />)
    await waitFor(() =>
      expect(screen.getByText(/eventually consistent/i)).toBeInTheDocument(),
    )
  })
})
