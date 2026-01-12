package localtest

import (
	"github.com/go-logr/logr"
	"github.com/posit-dev/team-operator/api/product"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakectrl "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type FakeTestEnv struct{}

func (fte *FakeTestEnv) Start(loadSchemes func(scheme *runtime.Scheme)) (client.WithWatch, *runtime.Scheme, logr.Logger) {
	cli := fakectrl.NewFakeClient()
	cliScheme := cli.Scheme()
	loadSchemes(cliScheme)

	log := product.NewSimpleLogger()
	return cli, cliScheme, log
}
