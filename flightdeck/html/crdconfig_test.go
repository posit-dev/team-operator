package html

import (
	"strings"
	"testing"

	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/flightdeck/internal"
	corev1 "k8s.io/api/core/v1"
)

func TestCRDConfigPage(t *testing.T) {
	// Create a sample Site with various configurations
	site := &positcov1beta1.Site{
		Spec: positcov1beta1.SiteSpec{
			Domain:               "example.com",
			AwsAccountId:         "123456789012",
			ClusterDate:          "20250101",
			WorkloadCompoundName: "test-workload",
			IngressClass:         "nginx",
			SharedDirectory:      "shared",
			PackageManagerUrl:    "https://pm.example.com",
			EFSEnabled:           true,
			VPCCIDR:              "10.0.0.0/16",
			Workbench: positcov1beta1.InternalWorkbenchSpec{
				Image:           "rstudio/workbench:latest",
				Replicas:        2,
				ImagePullPolicy: corev1.PullAlways,
			},
			Connect: positcov1beta1.InternalConnectSpec{
				Image:    "rstudio/connect:latest",
				Replicas: 3,
			},
			PackageManager: positcov1beta1.InternalPackageManagerSpec{
				Image:    "rstudio/pm:latest",
				Replicas: 1,
			},
		},
	}

	config := &internal.ServerConfig{
		SiteName:   "test-site",
		Namespace:  "posit-team",
		ShowConfig: true,
	}

	// Render the page
	page := CRDConfigPage(site, config)

	// Convert to string for testing
	var buf strings.Builder
	err := page.Render(&buf)
	if err != nil {
		t.Fatalf("Failed to render page: %v", err)
	}
	html := buf.String()

	// Check that the page contains expected content
	expectedStrings := []string{
		"Site Configuration",
		"auto-generated from the Site Custom Resource Definition",
		"example.com",
		"Basic Configuration",
		"Product Configuration",
		"workbench",
		"connect",
		"package Manager", // Note: formatting adds space
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(html, expected) {
			t.Errorf("Expected HTML to contain '%s', but it didn't", expected)
		}
	}

	// Check that zero values are not rendered
	unexpectedStrings := []string{
		"chronicle", // Not set, should be omitted
	}

	for _, unexpected := range unexpectedStrings {
		if strings.Contains(html, unexpected) {
			t.Errorf("Expected HTML to NOT contain '%s', but it did", unexpected)
		}
	}
}

func TestFormatFieldName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"awsAccountId", "aws Account Id"},
		{"VPCCIDR", "VPCCIDR"},
		{"packageManager", "package Manager"},
		{"enableFqdnHealthChecks", "enable Fqdn Health Checks"},
	}

	for _, test := range tests {
		result := formatFieldName(test.input)
		if result != test.expected {
			t.Errorf("formatFieldName(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestIsProductField(t *testing.T) {
	productFields := []string{"workbench", "connect", "packageManager", "chronicle", "flightdeck"}
	nonProductFields := []string{"domain", "secret", "debug"}

	for _, field := range productFields {
		if !isProductField(field) {
			t.Errorf("Expected %s to be a product field", field)
		}
	}

	for _, field := range nonProductFields {
		if isProductField(field) {
			t.Errorf("Expected %s to NOT be a product field", field)
		}
	}
}

func TestIsAdvancedField(t *testing.T) {
	advancedFields := []string{"secret", "workloadSecret", "debug", "networkTrust"}
	basicFields := []string{"domain", "awsAccountId", "clusterDate"}

	for _, field := range advancedFields {
		if !isAdvancedField(field) {
			t.Errorf("Expected %s to be an advanced field", field)
		}
	}

	for _, field := range basicFields {
		if isAdvancedField(field) {
			t.Errorf("Expected %s to NOT be an advanced field", field)
		}
	}
}
