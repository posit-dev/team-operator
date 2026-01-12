package localtest

import (
	"context"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/posit-dev/team-operator/api/product"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type LocalTestEnv struct {
	env *envtest.Environment
}

func (lte *LocalTestEnv) makeEnv() error {
	lte.env = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join(RootDir, "config", "crd", "bases"),
			filepath.Join(RootDir, "hack", "keycloak", "crds"),
			filepath.Join(RootDir, "hack", "traefik", "crds"),
		},
		ErrorIfCRDPathMissing: true,
	}

	return nil
}

func (lte *LocalTestEnv) Start(loadSchemes func(scheme *runtime.Scheme)) (client.Client, *runtime.Scheme, logr.Logger, error) {
	log := product.NewSimpleLogger()
	logf.SetLogger(log)
	if lte.env == nil {
		if err := lte.makeEnv(); err != nil {
			return nil, nil, log, err
		}
	}
	if cfg, err := lte.env.Start(); err != nil {
		return nil, nil, log, err
	} else {
		if cli, err := client.New(cfg, client.Options{}); err != nil {
			return cli, nil, log, err
		} else {
			cliScheme := cli.Scheme()
			loadSchemes(cliScheme)
			if err := cli.Create(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "posit-team"}}, &client.CreateOptions{}); err != nil {
				return cli, cliScheme, log, err
			} else {
				// success!!
				return cli, cliScheme, log, nil
			}
		}
	}
}

func (lte *LocalTestEnv) Stop() error {
	return lte.env.Stop()
}
