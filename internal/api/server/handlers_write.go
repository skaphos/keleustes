/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package server

import (
	"context"

	"github.com/skaphos/keleustes/internal/api/openapi"
)

// The promotion write surface is intentionally inert until the Git-mutation
// engine exists. Every desired-state change must land as a Git commit, never a
// live cluster or parameter override (ADR 0003), so these endpoints return 501
// rather than mutating anything. errNotImplemented carries that contract to the
// error handler, which maps it to not_implemented.

// PostPromotions would open a Git PR for a promotion request.
func (s *Server) PostPromotions(_ context.Context, _ openapi.PostPromotionsRequestObject) (openapi.PostPromotionsResponseObject, error) {
	return nil, errNotImplemented
}

// PostPromotionsIDApprove would record an approval decision and advance state.
func (s *Server) PostPromotionsIDApprove(_ context.Context, _ openapi.PostPromotionsIDApproveRequestObject) (openapi.PostPromotionsIDApproveResponseObject, error) {
	return nil, errNotImplemented
}

// PostPromotionsIDCancel would cancel an in-flight promotion.
func (s *Server) PostPromotionsIDCancel(_ context.Context, _ openapi.PostPromotionsIDCancelRequestObject) (openapi.PostPromotionsIDCancelResponseObject, error) {
	return nil, errNotImplemented
}

// PostPromotionsIDRetry would retry a failed promotion.
func (s *Server) PostPromotionsIDRetry(_ context.Context, _ openapi.PostPromotionsIDRetryRequestObject) (openapi.PostPromotionsIDRetryResponseObject, error) {
	return nil, errNotImplemented
}
