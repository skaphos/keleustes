// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { Link } from 'react-router-dom'
import { Button } from '@/components/ui/button'

export function NotFound() {
  return (
    <div className="flex flex-col items-start gap-4">
      <div>
        <h1 className="text-xl font-semibold">Not found</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          That route doesn’t exist in the scaffold yet.
        </p>
      </div>
      <Link to="/">
        <Button variant="outline" size="sm">
          Back to Overview
        </Button>
      </Link>
    </div>
  )
}
