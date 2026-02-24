// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;create;update;patch

package crdapply

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// permanentError wraps an error that should not be retried.
type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

//go:embed bases/*.yaml
var crdFiles embed.FS

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		panic(fmt.Errorf("registering apiextensions scheme: %w", err))
	}
}

// ApplyCRDs applies all embedded CRD manifests to the cluster using server-side apply.
// It is safe to call on every startup — SSA is idempotent and only updates when
// the schema actually differs from what is already in the cluster.
//
// ctx should carry a deadline; without one, a slow or unreachable API server will
// block the operator from starting indefinitely.
func ApplyCRDs(ctx context.Context, cfg *rest.Config, log logr.Logger) error {
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	return pollApplyCRDs(ctx, c, log, 5*time.Second, applyCRDs)
}

// pollApplyCRDs runs fn in a poll loop until it succeeds, fn returns a permanentError,
// or ctx is cancelled. Extracted for testability.
func pollApplyCRDs(ctx context.Context, c client.Client, log logr.Logger, interval time.Duration, fn func(context.Context, client.Client, logr.Logger) error) error {
	var lastErr error
	var isPermanent bool
	pollErr := wait.PollUntilContextCancel(ctx, interval, true, func(ctx context.Context) (bool, error) {
		if err := fn(ctx, c, log); err != nil {
			var pe permanentError
			if errors.As(err, &pe) {
				isPermanent = true
				return false, pe.err
			}
			log.Info("retrying CRD apply after transient error", "error", err)
			lastErr = err
			return false, nil
		}
		return true, nil
	})
	if pollErr != nil {
		if lastErr != nil && !isPermanent {
			return fmt.Errorf("%w; last apply error: %w", pollErr, lastErr)
		}
		return pollErr
	}
	return nil
}

// applyCRDs applies all embedded CRD manifests using the provided client.
// It collects all errors and returns them together to maximize the number of CRDs
// that get applied even if some fail transiently. SSA is atomic per-resource, so
// partial application is safe.
func applyCRDs(ctx context.Context, c client.Client, log logr.Logger) error {
	crds, err := ParseCRDs()
	if err != nil {
		return permanentError{err}
	}
	if len(crds) == 0 {
		return permanentError{fmt.Errorf("no CRDs found in embedded bases/; binary may have been built without running 'make copy-crds'")}
	}

	log.Info("applying CRDs with ForceOwnership", "hint", "if GitOps tooling (Flux, ArgoCD) manages your CRDs, set --manage-crds=false to avoid field-ownership conflicts")

	var errs []error
	for _, crd := range crds {
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
			errs = append(errs, fmt.Errorf("applying CRD %s: %w", crd.Name, err))
		} else {
			log.Info("applied CRD", "name", crd.Name)
		}
	}

	return errors.Join(errs...)
}

// ParseCRDs parses all embedded CRD manifests and returns them.
// Useful for testing that the embedded files are valid.
func ParseCRDs() ([]*apiextensionsv1.CustomResourceDefinition, error) {
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
