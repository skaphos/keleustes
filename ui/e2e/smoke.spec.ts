// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { test, expect } from '@playwright/test'

// End-to-end smoke against the dev server (Vite + MSW fixtures). Proves the
// shell boots, nav works, and the matrix renders contract data — the baseline
// usability path.
test('shell loads and navigates to the application matrix', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('KELEUSTES')).toBeVisible()

  await page.getByRole('link', { name: 'Applications' }).click()
  await expect(page).toHaveURL(/\/applications$/)

  // Matrix renders fixtures served by MSW.
  await expect(page.getByText('api')).toBeVisible()
  await expect(page.getByText(/eventually consistent/i)).toBeVisible()
})

test('navigates to the promotions queue', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Promotions' }).click()
  await expect(page).toHaveURL(/\/promotions$/)
  await expect(page.getByText('Blocked')).toBeVisible()
})
