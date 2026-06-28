// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'
import { StatusBadge } from '@/components/StatusBadge'
import type { Status } from '@/lib/status'

// Promotions list + approvals queue (ui-design-spec §6.4). Data-backed.
export function Promotions() {
  const { data, isLoading } = useQuery({
    queryKey: ['promotions'],
    queryFn: async () => {
      const { data, error } = await api.GET('/promotions', {})
      if (error) throw new Error('failed to load promotions')
      return data
    },
  })

  return (
    <div>
      <PageHeader
        title="Promotions"
        description="Proposed moves of a release between environments — gates, approvals, audit."
      />
      {isLoading && <div className="h-40 animate-pulse rounded-lg bg-muted" aria-label="Loading" />}
      <div className="overflow-hidden rounded-lg border">
        <table className="w-full text-sm">
          <thead className="bg-card text-left text-muted-foreground">
            <tr>
              <th className="px-4 py-2 font-medium">Application</th>
              <th className="px-4 py-2 font-medium">Move</th>
              <th className="px-4 py-2 font-medium">Release</th>
              <th className="px-4 py-2 font-medium">Status</th>
              <th className="px-4 py-2 font-medium">Requested by</th>
            </tr>
          </thead>
          <tbody>
            {data?.map((p) => (
              <tr key={p.ulid} className="border-t hover:bg-accent/40">
                <td className="px-4 py-2 font-medium">
                  <Link className="hover:underline" to={`/promotions/${p.ulid}`}>
                    {p.application}
                  </Link>
                </td>
                <td className="px-4 py-2 font-mono text-xs">
                  {p.from} → {p.to}
                </td>
                <td className="px-4 py-2 font-mono text-xs">{p.release}</td>
                <td className="px-4 py-2">
                  <StatusBadge status={p.status as Status} />
                </td>
                <td className="px-4 py-2 text-muted-foreground">{p.requestedBy}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
