package internal

import (
	"context"

	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockSiteClient implements SiteInterface with mock data for local development
type mockSiteClient struct{}

// NewMockSiteClient returns a SiteInterface that returns mock data
func NewMockSiteClient() SiteInterface {
	return &mockSiteClient{}
}

func (m *mockSiteClient) Get(name string, namespace string, opts metav1.GetOptions, ctx context.Context) (*positcov1beta1.Site, error) {
	return &positcov1beta1.Site{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: positcov1beta1.SiteSpec{
			Domain: "example.posit.team",
			Workbench: positcov1beta1.InternalWorkbenchSpec{
				DomainPrefix: "workbench",
				Image:        "ghcr.io/rstudio/rstudio-workbench:2024.04.0",
			},
			Connect: positcov1beta1.InternalConnectSpec{
				DomainPrefix: "connect",
				Image:        "ghcr.io/rstudio/rstudio-connect:2024.03.0",
			},
			PackageManager: positcov1beta1.InternalPackageManagerSpec{
				DomainPrefix: "packagemanager",
				Image:        "ghcr.io/rstudio/rstudio-pm:2024.04.0",
			},
		},
	}, nil
}
