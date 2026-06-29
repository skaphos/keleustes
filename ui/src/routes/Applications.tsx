// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'
import { statusMeta, type Status } from '@/lib/status'
import { cn } from '@/lib/utils'

// The Application matrix (ui-design-spec §6.2) — the centerpiece. Data-backed
// to prove the contract → client → Query → MSW pipeline end to end.
function useMatrix() {
  return useQuery({
    queryKey: ['matrix'],
    queryFn: async () => {
      const { data, error } = await api.GET('/applications/{name}/matrix', {
        params: { path: { name: 'all' } },
      })
      if (error) throw new Error('failed to load matrix')
      return data
    },
  })
}

export function Applications() {
  const { data, isLoading, isError } = useMatrix()

  return (
    <div>
      <PageHeader
        title="Applications"
        description="Deployed version and health for every application across environments and regions."
      />

      {data?.asOf && (
        <p className="mb-3 text-xs text-muted-foreground">
          as of {new Date(data.asOf).toLocaleTimeString()} · snapshot is eventually consistent
        </p>
      )}

      {isLoading && <div className="h-40 animate-pulse rounded-lg bg-muted" aria-label="Loading" />}
      {isError && (
        <div className="rounded-md border border-status-failed/40 p-4 text-sm text-status-failed">
          Failed to load the matrix. Retry.
        </div>
      )}

      {data && (
        <div className="overflow-auto rounded-lg border">
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="bg-card">
                <th className="sticky left-0 z-10 bg-card px-4 py-2 text-left font-medium">
                  Application
                </th>
                {data.columns?.map((c) => (
                  <th key={`${c.env}-${c.region}`} className="px-4 py-2 text-left font-medium">
                    {c.env}-{c.region}
                    {c.lagging && <span className="ml-1 text-status-drifted">(lagging)</span>}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {data.rows?.map((row) => (
                <tr key={row.application} className="border-t">
                  <td className="sticky left-0 z-10 bg-background px-4 py-2 font-medium">
                    <Link className="hover:underline" to={`/applications/${row.application}`}>
                      {row.application}
                    </Link>
                  </td>
                  {row.cells?.map((cell, i) => {
                    const meta = statusMeta(cell.status as Status)
                    return (
                      <td key={i} className="px-4 py-2">
                        <span className="inline-flex items-center gap-1.5 font-mono">
                          <span className={cn('text-base leading-none', meta.color)} aria-hidden>
                            {meta.glyph}
                          </span>
                          <span>{cell.version}</span>
                          {/* Status conveyed by color/glyph — announce it for screen readers. */}
                          <span className="sr-only">
                            {meta.label}
                            {cell.drift ? ', drifted' : ''}
                          </span>
                          {cell.drift && (
                            <span className="text-status-drifted" aria-hidden>
                              ⤳
                            </span>
                          )}
                        </span>
                      </td>
                    )
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
