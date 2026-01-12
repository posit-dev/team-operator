package product

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SomeReconciler interface {
	client.Reader
	client.Writer
	client.StatusClient
	GetLogger(ctx context.Context) logr.Logger
}

type FakeReconciler struct {
}

func (t *FakeReconciler) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return nil
}

func (t *FakeReconciler) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}
func (t *FakeReconciler) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return nil
}

func (t *FakeReconciler) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return nil
}

func (t *FakeReconciler) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return nil
}

func (t *FakeReconciler) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

func (t *FakeReconciler) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
	return nil
}

func (t *FakeReconciler) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

func (t *FakeReconciler) Status() client.SubResourceWriter {
	return FakeSubResourceWriter{}
}

func (t *FakeReconciler) GetLogger(ctx context.Context) logr.Logger {
	return logr.Logger{}
}

type FakeSubResourceWriter struct {
}

func (t FakeSubResourceWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return nil
}

func (t FakeSubResourceWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return nil
}

func (t FakeSubResourceWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return nil
}
