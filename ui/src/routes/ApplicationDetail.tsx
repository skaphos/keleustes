// SPDX-FileCopyrightText: 2026 Rillan AI LLC
// SPDX-License-Identifier: MIT
import { useParams, Link } from 'react-router-dom'
import { PageHeader, ScreenStub } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'

export function ApplicationDetail() {
  const { appName } = useParams()
  return (
    <div>
      <PageHeader
        title={`Application: ${appName}`}
        description="Targets · health · drift · promotion history · resources · diff · audit."
        actions={
          <>
            <Link to={`/applications/${appName}/diff`}>
              <Button variant="outline" size="sm">
                View diff
              </Button>
            </Link>
            {/* Promote = opens a Git PR (ADR 0003). Verb-gated in the real impl. */}
            <Button size="sm">Promote…</Button>
          </>
        }
      />
      <ScreenStub specRef="§6.3 Application detail" />
    </div>
  )
}
