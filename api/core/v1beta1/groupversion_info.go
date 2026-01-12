// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

// Package v1beta1 contains API Schema definitions for the  v1beta1 API group
// +kubebuilder:object:generate=true
// +groupName=core.posit.team
package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "core.posit.team", Version: "v1beta1"}

	// SchemeGroupVersion is alias to GroupVersion for client-go libraries.
	// It is required by pkg/client/informers/externalversions/...
	SchemeGroupVersion = GroupVersion

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// Resource is required by pkg/client/listers/...
func Resource(resource string) schema.GroupResource {
	return GroupVersion.WithResource(resource).GroupResource()
}
