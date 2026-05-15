/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

// Package v1alpha1 contains API Schema definitions for the keleustes v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=keleustes.skaphos.dev
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersion is the group/version used to register these objects.
var GroupVersion = schema.GroupVersion{Group: "keleustes.skaphos.dev", Version: "v1alpha1"}

// builder is a minimal scheme builder bound to GroupVersion. It mirrors the
// controller-runtime scheme.Builder API but depends only on apimachinery so the
// api package stays lightweight.
type builder struct {
	groupVersion schema.GroupVersion
	runtime.SchemeBuilder
}

// Register schedules the given objects to be added to a scheme under GroupVersion.
func (b *builder) Register(objects ...runtime.Object) *builder {
	b.SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(b.groupVersion, objects...)
		metav1.AddToGroupVersion(s, b.groupVersion)
		return nil
	})
	return b
}

var (
	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &builder{groupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
