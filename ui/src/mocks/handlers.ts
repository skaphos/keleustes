// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { http, HttpResponse } from 'msw'
import * as fx from './fixtures'

const base = '/api/v1'

export const handlers = [
  http.get(`${base}/applications`, () => HttpResponse.json(fx.applications)),
  http.get(`${base}/applications/:name`, ({ params }) => {
    const app = fx.applications.items.find((a) => a.name === params.name)
    return app ? HttpResponse.json(app) : new HttpResponse(null, { status: 404 })
  }),
  http.get(`${base}/applications/:name/matrix`, () => HttpResponse.json(fx.matrix)),
  http.get(`${base}/applications/:name/releases`, ({ params }) =>
    HttpResponse.json(fx.releases.filter((r) => r.app === params.name)),
  ),
  http.get(`${base}/applications/:name/promotions`, ({ params }) =>
    HttpResponse.json(fx.promotions.filter((p) => p.application === params.name)),
  ),

  http.get(`${base}/promotions`, () => HttpResponse.json(fx.promotions)),
  http.get(`${base}/promotions/:id`, ({ params }) => {
    const p = fx.promotions.find((x) => x.ulid === params.id)
    return p ? HttpResponse.json(p) : new HttpResponse(null, { status: 404 })
  }),

  http.get(`${base}/releases`, () => HttpResponse.json(fx.releases)),
  http.get(`${base}/environments`, () => HttpResponse.json(fx.environments)),
  http.get(`${base}/audit`, () => HttpResponse.json(fx.audit)),
]
