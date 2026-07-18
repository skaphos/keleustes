/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

// Package crdsource is the pre-scale [readmodel.ReadModel] adapter: it serves
// the API server's read surface by listing and getting the keleustes.skaphos.io
// CRDs through a controller-runtime client and mapping them into the
// product-concept openapi types.
//
// ADR 0005 §10 sanctions live CRD reads as the correct baseline until scale
// demands the NATS-KV hot index. Addressing is by natural key
// (metadata.name — ADR 0008); the port carries no namespace, so getters list
// cluster-wide and match on name.
//
// Keleustes is at the scaffold stage: reconcilers only set ObservedGeneration
// and an Accepted condition, so engine-derived data (per-cell sync state, drift,
// health detail, artifact versions, durable ULIDs, audit history) is not yet in
// CRD status. This adapter maps what exists today and leaves the rest as sparse
// but schema-valid zero values; each gap is commented at its mapping site and
// fills in as the Source/Sync/Promotion/Diff/Health engines land.
package crdsource
