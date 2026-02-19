package localtest

import (
	"github.com/go-logr/logr"
	v1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakectrl "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type FakeTestEnv struct{}

func (fte *FakeTestEnv) Start(loadSchemes func(scheme *runtime.Scheme)) (client.WithWatch, *runtime.Scheme, logr.Logger) {
	scheme := runtime.NewScheme()
	loadSchemes(scheme)

	cli := fakectrl.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(
			&v1beta1.Connect{},
			&v1beta1.Workbench{},
			&v1beta1.PackageManager{},
			&v1beta1.Chronicle{},
			&v1beta1.Flightdeck{},
			&v1beta1.PostgresDatabase{},
			&v1beta1.Site{},
		).
		Build()

	log := product.NewSimpleLogger()
	return cli, scheme, log
}
