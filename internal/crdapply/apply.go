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

// ApplyCRDs applies all embedded CRD manifests to the cluster using server-side apply.
// It is safe to call on every startup — SSA is idempotent and only updates when
// the schema actually differs from what is already in the cluster.
func ApplyCRDs(cfg *rest.Config, log logr.Logger) error {
	scheme := runtime.NewScheme()
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("registering apiextensions scheme: %w", err)
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	codec := serializer.NewCodecFactory(scheme)
	entries, err := fs.ReadDir(crdFiles, "bases")
	if err != nil {
		return fmt.Errorf("reading embedded CRD directory: %w", err)
	}

	var firstErr error
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := crdFiles.ReadFile("bases/" + entry.Name())
		if err != nil {
			log.Error(err, "failed to read embedded CRD", "file", entry.Name())
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		obj, _, err := codec.UniversalDeserializer().Decode(data, nil, nil)
		if err != nil {
			log.Error(err, "failed to decode CRD", "file", entry.Name())
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
		if !ok {
			log.Error(fmt.Errorf("unexpected type %T", obj), "not a CRD", "file", entry.Name())
			continue
		}

		// Explicitly set TypeMeta for SSA
		crd.TypeMeta = metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		}

		// Server-side apply — idempotent, only patches what changed
		patch := client.Apply
		if err := c.Patch(context.Background(), crd, patch,
			client.ForceOwnership,
			client.FieldOwner("team-operator"),
		); err != nil {
			log.Error(err, "failed to apply CRD", "name", crd.Name)
			if firstErr == nil {
				firstErr = err
			}
		} else {
			log.Info("applied CRD", "name", crd.Name)
		}
	}

	return firstErr
}

// ParseCRDs parses all embedded CRD manifests and returns them.
// Useful for testing that the embedded files are valid.
func ParseCRDs() ([]*apiextensionsv1.CustomResourceDefinition, error) {
	scheme := runtime.NewScheme()
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("registering apiextensions scheme: %w", err)
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
