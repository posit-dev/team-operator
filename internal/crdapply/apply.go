// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package crdapply

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed bases/*.yaml
var crdFiles embed.FS

func newScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("registering apiextensions scheme: %w", err)
	}
	return scheme, nil
}

// ApplyCRDs applies all embedded CRD manifests to the cluster using server-side apply.
// It is safe to call on every startup — SSA is idempotent and only updates when
// the schema actually differs from what is already in the cluster.
//
// ctx should carry a deadline; without one, a slow or unreachable API server will
// block the operator from starting indefinitely.
func ApplyCRDs(ctx context.Context, cfg *rest.Config, log logr.Logger) error {
	scheme, err := newScheme()
	if err != nil {
		return err
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	return applyCRDs(ctx, c, log)
}

// applyCRDs applies all embedded CRD manifests using the provided client.
// It fails fast on the first error to avoid leaving the cluster in a partially-updated state.
func applyCRDs(ctx context.Context, c client.Client, log logr.Logger) error {
	scheme, err := newScheme()
	if err != nil {
		return err
	}

	codec := serializer.NewCodecFactory(scheme)
	entries, err := fs.ReadDir(crdFiles, "bases")
	if err != nil {
		return fmt.Errorf("reading embedded CRD directory: %w", err)
	}

	log.Info("applying CRDs with ForceOwnership; if GitOps tooling (Flux, ArgoCD) manages your CRDs, set --manage-crds=false to avoid field-ownership conflicts")

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := crdFiles.ReadFile("bases/" + entry.Name())
		if err != nil {
			return fmt.Errorf("reading embedded CRD %s: %w", entry.Name(), err)
		}

		obj, _, err := codec.UniversalDeserializer().Decode(data, nil, nil)
		if err != nil {
			return fmt.Errorf("decoding CRD %s: %w", entry.Name(), err)
		}

		crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
		if !ok {
			return fmt.Errorf("unexpected type %T in %s", obj, entry.Name())
		}

		// Explicitly set TypeMeta for SSA
		crd.TypeMeta = metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		}

		// Server-side apply — idempotent, only patches what changed
		if err := c.Patch(ctx, crd, client.Apply,
			client.ForceOwnership,
			client.FieldOwner("team-operator"),
		); err != nil {
			return fmt.Errorf("applying CRD %s: %w", crd.Name, err)
		}
		log.Info("applied CRD", "name", crd.Name)
	}

	return nil
}

// ParseCRDs parses all embedded CRD manifests and returns them.
// Useful for testing that the embedded files are valid.
func ParseCRDs() ([]*apiextensionsv1.CustomResourceDefinition, error) {
	scheme, err := newScheme()
	if err != nil {
		return nil, err
	}

	codec := serializer.NewCodecFactory(scheme)
	entries, err := fs.ReadDir(crdFiles, "bases")
	if err != nil {
		return nil, fmt.Errorf("reading embedded CRD directory: %w", err)
	}

	var crds []*apiextensionsv1.CustomResourceDefinition
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := crdFiles.ReadFile("bases/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}

		obj, _, err := codec.UniversalDeserializer().Decode(data, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("decoding %s: %w", entry.Name(), err)
		}

		crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
		if !ok {
			return nil, fmt.Errorf("unexpected type %T in %s", obj, entry.Name())
		}
		crds = append(crds, crd)
	}
	return crds, nil
}
