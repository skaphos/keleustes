// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
//
// Realistic fixtures that satisfy openapi/keleustes.v1.yaml. One dataset powers
// both `pnpm dev` (usability) and the test suite. When the Go API server lands,
// these become contract fixtures to test it against.
import type { Status } from '@/lib/status'

// targetRef is per (app, env, region) so fixtures stay realistic once it drives
// navigation/popovers.
const cell = (
  app: string,
  env: string,
  region: string,
  version: string,
  status: Status,
  drift = false,
) => ({
  env,
  region,
  targetRef: `${env}-${region}/${app}`,
  version,
  status,
  drift,
  lastSync: '2026-06-28T14:30:00Z',
})

export const matrix = {
  asOf: '2026-06-28T14:32:07Z',
  columns: [
    { env: 'dev', region: 'us', lagging: false },
    { env: 'staging', region: 'us', lagging: false },
    { env: 'prod', region: 'us', lagging: false },
    { env: 'prod', region: 'eu', lagging: false },
  ],
  rows: [
    {
      application: 'api',
      cells: [
        cell('api', 'dev', 'us', '1.4.2', 'Healthy'),
        cell('api', 'staging', 'us', '1.4.2', 'Healthy'),
        cell('api', 'prod', 'us', '1.4.1', 'Progressing'),
        cell('api', 'prod', 'eu', '1.3.9', 'Degraded', true),
      ],
    },
    {
      application: 'web',
      cells: [
        cell('web', 'dev', 'us', '2.0.0', 'Healthy'),
        cell('web', 'staging', 'us', '2.0.0', 'Healthy'),
        cell('web', 'prod', 'us', '1.9.4', 'Healthy'),
        cell('web', 'prod', 'eu', '1.9.4', 'Healthy'),
      ],
    },
    {
      application: 'worker',
      cells: [
        cell('worker', 'dev', 'us', '0.7.1', 'Healthy'),
        cell('worker', 'staging', 'us', '0.0.0', 'Missing'),
        cell('worker', 'prod', 'us', '0.0.0', 'Missing'),
        cell('worker', 'prod', 'eu', '0.0.0', 'Missing'),
      ],
    },
  ],
}

export const applications = {
  asOf: '2026-06-28T14:32:07Z',
  items: [
    {
      ulid: '01J0AAA0000000000000000API',
      name: 'api',
      project: 'payments',
      owner: 'team-payments',
      status: 'Degraded' as Status,
      source: { repo: 'git@github.com:acme/api', path: 'deploy', ref: 'main', commit: 'abc123d' },
      addonRefs: [],
    },
    {
      ulid: '01J0AAA0000000000000000WEB',
      name: 'web',
      project: 'storefront',
      owner: 'team-web',
      status: 'Healthy' as Status,
      source: { repo: 'git@github.com:acme/web', path: 'deploy', ref: 'main', commit: 'def456a' },
      addonRefs: [],
    },
    {
      ulid: '01J0AAA000000000000000WRK',
      name: 'worker',
      project: 'payments',
      owner: 'team-payments',
      status: 'Healthy' as Status,
      source: { repo: 'git@github.com:acme/worker', path: 'deploy', ref: 'main', commit: '789beef' },
      addonRefs: [],
    },
  ],
}

export const promotions = [
  {
    ulid: '01J0PROM00000000000000001',
    application: 'api',
    from: 'staging',
    to: 'prod',
    mode: 'standard',
    release: '1.4.2',
    status: 'Blocked' as Status,
    requestedBy: 'alice',
    requestedAt: '2026-06-28T14:01:00Z',
    prUrl: '',
    gates: [
      { name: 'image-signed', passed: true, detail: 'cosign verified' },
      { name: 'vuln-scan', passed: true, detail: 'no criticals' },
      { name: 'approvals', passed: false, detail: '1 of 2 approvals' },
    ],
    approvals: [
      { approver: 'alice', decision: 'approve', comment: 'LGTM', at: '2026-06-28T14:02:00Z' },
      { approver: 'bob', decision: 'pending', comment: '', at: '' },
    ],
    sync: 'Pending',
  },
  {
    ulid: '01J0PROM00000000000000002',
    application: 'web',
    from: 'dev',
    to: 'staging',
    mode: 'standard',
    release: '2.0.0',
    status: 'Progressing' as Status,
    requestedBy: 'carol',
    requestedAt: '2026-06-28T13:40:00Z',
    prUrl: 'https://github.com/acme/web/pull/318',
    gates: [{ name: 'image-signed', passed: true, detail: 'cosign verified' }],
    approvals: [],
    sync: 'Running',
  },
]

export const releases = [
  {
    ulid: '01J0REL000000000000000142',
    version: '1.4.2',
    app: 'api',
    source: { repo: 'git@github.com:acme/api', path: 'deploy', ref: 'main', commit: 'abc123d' },
    created: '2026-06-28T12:00:00Z',
    provenance: { signed: true, sbom: true, attestation: true },
    deployedOn: ['dev-us/api', 'staging-us/api'],
  },
  {
    ulid: '01J0REL000000000000000200',
    version: '2.0.0',
    app: 'web',
    source: { repo: 'git@github.com:acme/web', path: 'deploy', ref: 'main', commit: 'def456a' },
    created: '2026-06-28T11:00:00Z',
    provenance: { signed: true, sbom: false, attestation: false },
    deployedOn: ['dev-us/web', 'staging-us/web'],
  },
]

export const environments = [
  {
    name: 'prod',
    regions: ['us', 'eu'],
    cells: [
      {
        name: 'prod-us-1',
        region: 'us',
        status: 'Healthy' as Status,
        targets: [
          {
            name: 'prod-us/api',
            env: 'prod',
            region: 'us',
            cell: 'prod-us-1',
            cluster: 'gke-prod-us-1',
            version: '1.4.1',
            status: 'Progressing' as Status,
            drift: false,
            frozen: false,
            lastSync: '2026-06-28T14:30:00Z',
          },
        ],
      },
      {
        name: 'prod-eu-1',
        region: 'eu',
        status: 'Degraded' as Status,
        targets: [
          {
            name: 'prod-eu/api',
            env: 'prod',
            region: 'eu',
            cell: 'prod-eu-1',
            cluster: 'gke-prod-eu-1',
            version: '1.3.9',
            status: 'Degraded' as Status,
            drift: true,
            frozen: true,
            lastSync: '2026-06-28T13:55:00Z',
          },
        ],
      },
    ],
    freeze: {
      active: true,
      scope: 'prod-eu',
      start: '2026-06-28T00:00:00Z',
      end: '2026-06-29T00:00:00Z',
      reason: 'EU change freeze (quarter close)',
    },
  },
]

export const audit = {
  items: [
    {
      ulid: '01J0AUD000000000000000003',
      actor: 'alice',
      verb: 'approve',
      target: 'Promotion 01J0PROM00000000000000001 (api staging→prod)',
      at: '2026-06-28T14:03:00Z',
      result: { before: { approvals: 0 }, after: { approvals: 1 } },
      evidence: ['cosign:verified', 'vuln:clean'],
    },
    {
      ulid: '01J0AUD000000000000000002',
      actor: 'alice',
      verb: 'promote',
      target: 'Application api',
      at: '2026-06-28T14:01:00Z',
      result: { before: { env: 'staging' }, after: { env: 'prod (requested)' } },
      evidence: [],
    },
    {
      ulid: '01J0AUD000000000000000001',
      actor: 'system',
      verb: 'sync',
      target: 'DeploymentTarget prod-us/api',
      at: '2026-06-28T13:58:00Z',
      result: { before: { version: '1.4.0' }, after: { version: '1.4.1' } },
      evidence: [],
    },
  ],
}
