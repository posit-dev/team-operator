package internal

import (
	"context"
	"os"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	watchNamespacesEnvVar = "WATCH_NAMESPACES"
)

// GetWatchNamespaces returns the Namespaces the operator should be watching for changes
//
// This function is (lovingly!) lifted/borrowed/modified from
// https://sdk.operatorframework.io/docs/building-operators/golang/operator-scope/#configuring-watch-namespaces-dynamically
func GetWatchNamespaces() map[string]cache.Config {
	c := map[string]cache.Config{}

	allNs, found := os.LookupEnv(watchNamespacesEnvVar)
	if found {
		for _, ns := range strings.Split(allNs, ",") {
			c[strings.TrimSpace(ns)] = cache.Config{}
		}

		return c
	}

	c[product.PositTeamNamespace] = cache.Config{}

	return c
}

// BasicDelete is much like the analog BasicCreateOrUpdate, except it only deletes the object. It also checks labels to be sure
// that the object is managed by the team-operator before purging it.
func BasicDelete(ctx context.Context, r product.SomeReconciler, l logr.Logger, key client.ObjectKey, existingObj client.Object) error {
	kind := reflect.TypeOf(existingObj).String()
	name := key.String()
	l = l.WithValues(
		"event", "delete",
		"object", kind,
		"name", name,
	)
	if err := r.Get(ctx, key, existingObj); err != nil && errors.IsNotFound(err) {
		// not found, do nothing
		l.Info("object not found; doing nothing")
	} else if err != nil {
		// unknown error
		l.Error(err, "unknown error occurred retrieving object")
		return err
	} else {
		// check labels
		labels := existingObj.GetLabels()
		if labels[v1beta1.ManagedByLabelKey] != v1beta1.ManagedByLabelValue {
			noManagedError := errors.NewUnauthorized("object not managed by " + v1beta1.ManagedByLabelValue)
			l.Error(noManagedError, "object not managed by "+v1beta1.ManagedByLabelValue+"; delete failed")
			return noManagedError
		}

		// clean up
		if err := r.Delete(ctx, existingObj); err != nil {
			l.Error(err, "error occurred while deleting object")
			return err
		}
	}
	return nil
}

// PvcCreateOrUpdate is careful only to patch valid fields _if they have changed_. Otherwise, leave things
// alone! In particular, StorageClassName will throw a diff every time if we leave it as blank (the default), because
// Kubernetes fills in the StorageClassName
func PvcCreateOrUpdate(
	ctx context.Context,
	r product.SomeReconciler,
	l logr.Logger,
	key client.ObjectKey,
	existingPvc *v1.PersistentVolumeClaim,
	targetPvc *v1.PersistentVolumeClaim,
) error {
	kind := reflect.TypeOf(existingPvc).String()
	name := key.String()

	l = l.WithValues(
		"event", "create-or-update",
		"object", kind,
		"name", name,
	)

	if err := r.Get(ctx, key, existingPvc); err != nil && errors.IsNotFound(err) {
		l.Info("creating object")

		if err := r.Create(ctx, targetPvc); err != nil {
			l.Error(err, "error occurred creating the object")

			return err
		}
	} else if err != nil {
		l.Error(err, "unknown error occurred retrieving the object")

		return err
	} else {
		if existingPvc.Spec.Resources.Requests.Storage().String() != targetPvc.Spec.Resources.Requests.Storage().String() {
			l.Info("found existing object; storage differs; we do not yet support patching")
			// TODO: patching unfortunately is highly complex with volumes
			//   - you can only patch bound claims
			//   - the StorageClass must support resizing volumes dynamically
			//   For now, we will leave this alone and come back to it later.
		} else {
			l.Info("found existing object; no storage change, so not patching")
		}
	}

	return nil
}

// BasicCreateNoUpdate is useful when no updates are necessary. We need to only create a resource
func BasicCreateNoUpdate(
	ctx context.Context,
	r product.SomeReconciler,
	l logr.Logger,
	key client.ObjectKey,
	existingObj client.Object,
	targetObj client.Object,
) error {
	kind := reflect.TypeOf(existingObj).String()
	name := key.String()

	l = l.WithValues(
		"event", "create-no-update",
		"object", kind,
		"name", name,
	)

	l.V(9).WithValues("key", key).Info("Attempting to get object")

	if err := r.Get(ctx, key, existingObj); err != nil && errors.IsNotFound(err) {
		if targetObj.GetLabels()[v1beta1.ManagedByLabelKey] != v1beta1.ManagedByLabelValue {
			noManagedError := errors.NewUnauthorized("object to create not managed by " + v1beta1.ManagedByLabelValue)

			l.Error(noManagedError, "object to create not managed by "+v1beta1.ManagedByLabelValue+"; create failed")

			return noManagedError
		}

		l.Info("creating object")

		if err := r.Create(ctx, targetObj); err != nil {
			l.Error(err, "error occurred creating the object")

			return err
		}
	} else if err != nil {
		l.Error(err, "unknown error occurred retrieving the object")

		return err
	} else {
		l.Info("found existing object; not updating")

		labels := existingObj.GetLabels()

		// a missing key in a map will be treated as "" and satisfy this !=
		// so we do not have to check for the key missing
		if labels[v1beta1.ManagedByLabelKey] != v1beta1.ManagedByLabelValue {
			noManagedError := errors.NewUnauthorized("object not managed by " + v1beta1.ManagedByLabelValue)

			l.Error(noManagedError, "object not managed by "+v1beta1.ManagedByLabelValue+"; update failed")

			return noManagedError
		}

		l.Info("we are not updating this resource because we do not currently handle updates")
	}

	return nil
}

// BasicCreateOrUpdate is useful when there is no custom logic to handle during the update process
// It creates the object if it does not exist, updates universally if it does, and returns an
// error if it encounters anything it does not expect.
// We expect that it will be replaced or extended as we become more familiar with the service and its needs.
func BasicCreateOrUpdate(ctx context.Context, r product.SomeReconciler, l logr.Logger, key client.ObjectKey, existingObj client.Object, targetObj client.Object) error {
	kind := reflect.TypeOf(existingObj).String()
	name := key.String()
	l = l.WithValues(
		"event", "create-or-update",
		"object", kind,
		"name", name,
	)
	if err := r.Get(ctx, key, existingObj); err != nil && errors.IsNotFound(err) {
		// not found, create it
		if targetObj.GetLabels()[v1beta1.ManagedByLabelKey] != v1beta1.ManagedByLabelValue {
			noManagedError := errors.NewUnauthorized("object to create not managed by " + v1beta1.ManagedByLabelValue)
			l.Error(noManagedError, "object to create not managed by "+v1beta1.ManagedByLabelValue+"; create failed")
			return noManagedError
		}
		l.Info("creating object")
		if err := r.Create(ctx, targetObj); err != nil {
			l.Error(err, "error occurred creating the object")
			return err
		}
	} else if err != nil {
		// unknown error
		l.Error(err, "unknown error occurred retrieving the object")
		return err
	} else {
		// already exists, update
		l.Info("found existing object; updating", "name", existingObj.GetName(), "kind", existingObj.GetObjectKind())
		labels := existingObj.GetLabels()

		// a missing key in a map will be treated as "" and satisfy this !=
		// so we do not have to check for the key missing
		if labels[v1beta1.ManagedByLabelKey] != v1beta1.ManagedByLabelValue {
			noManagedError := errors.NewUnauthorized("object not managed by " + v1beta1.ManagedByLabelValue)
			l.Error(noManagedError, "object not managed by "+v1beta1.ManagedByLabelValue+"; update failed")
			return noManagedError
		}

		targetObj.SetResourceVersion(existingObj.GetResourceVersion())
		if err := r.Update(ctx, targetObj); err != nil {
			l.Error(err, "error occurred updating object")
			return err
		}
	}
	return nil
}
