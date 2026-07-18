/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

// Package readmodel defines the interaction layer between the Keleustes API
// server (internal/api/server) and the data that backs the read surface of the
// contract (openapi/keleustes.v1.yaml).
//
// The package exports a single port, [ReadModel], that the HTTP handlers — and,
// through the generated client, keleustesctl — depend on. The port returns the
// generated openapi model types so there is exactly one wire shape; it is not a
// parallel domain layer.
//
// Two adapters satisfy the port today:
//
//   - crdsource: reads the keleustes.skaphos.io CRDs through a controller-runtime
//     client. This is the sanctioned pre-scale baseline — ADR 0005 §10 describes
//     the NATS KV deployment-snapshots bucket as "an alternative to live CRD list
//     at scale," i.e. live CRD reads are correct until scale demands the hot index.
//   - fixtures: an in-memory adapter that mirrors the UI's mock fixtures
//     (ui/src/mocks/fixtures.ts) so the server runs end-to-end with no cluster and
//     the UI's fixtures double as the contract-test corpus.
//
// The eventual NATS-KV + DuckDB-on-parquet read path (ADR 0005 §13) is a third
// adapter behind this same interface; handlers and the CLI do not change when it
// lands.
package readmodel
