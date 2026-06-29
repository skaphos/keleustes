// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { useParams } from 'react-router-dom'
import { PageHeader, ScreenStub } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'

export function PromotionDetail() {
  const { promotionUlid } = useParams()
  return (
    <div>
      <PageHeader
        title={`Promotion: ${promotionUlid}`}
        description="Timeline: request → gates → approvals → Git mutation → sync."
        actions={
          <>
            {/* The three write actions (ADR 0003 §6) — verb-gated in real impl. */}
            <Button variant="outline" size="sm">
              Reject
            </Button>
            <Button size="sm">Approve</Button>
          </>
        }
      />
      <ScreenStub specRef="§6.4 Promotion timeline" />
    </div>
  )
}
