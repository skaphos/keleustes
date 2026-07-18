// SPDX-FileCopyrightText: 2026 Rillan AI LLC
// SPDX-License-Identifier: MIT
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'

// Audit / activity stream (ui-design-spec §6.7, SKA-348). Data-backed.
export function Audit() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['audit'],
    queryFn: async () => {
      const { data, error } = await api.GET('/audit', {})
      if (error) throw new Error('failed to load audit')
      return data
    },
  })

  return (
    <div>
      <PageHeader
        title="Audit"
        description="What changed, when, by whom — append-only activity from the event log."
      />
      {isLoading && <div className="h-40 animate-pulse rounded-lg bg-muted" aria-label="Loading" />}
      {isError && (
        <div className="rounded-md border border-status-failed/40 p-4 text-sm text-status-failed">
          Failed to load the audit log. Retry.
        </div>
      )}
      <ul className="divide-y rounded-lg border">
        {data?.items?.map((e) => (
          <li key={e.ulid} className="flex items-baseline gap-3 px-4 py-2 text-sm">
            <time className="w-20 shrink-0 font-mono text-xs text-muted-foreground">
              {new Date(e.at).toLocaleTimeString()}
            </time>
            <span className="w-16 shrink-0 font-medium">{e.actor}</span>
            <span className="w-24 shrink-0 text-muted-foreground">{e.verb}</span>
            <span className="min-w-0 flex-1 truncate">{e.target}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}
