// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CommonProductStatus contains the common status fields shared by all product CRDs.
// Embed this struct inline in product-specific Status types.
type CommonProductStatus struct {
	// Conditions represent the latest available observations of the resource's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ObservedGeneration is the most recent generation observed for this resource.
	// It corresponds to the resource's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Version is the version of the product image being deployed.
	// +optional
	Version string `json:"version,omitempty"`
}
