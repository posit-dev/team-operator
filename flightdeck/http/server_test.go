package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/flightdeck/internal"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mock implementation of SiteInterface
type mockSiteClient struct {
	site *v1beta1.Site
	err  error
}

func (m *mockSiteClient) Get(name string, namespace string, options metav1.GetOptions, ctx context.Context) (*v1beta1.Site, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.site, nil
}

func TestHealthEndpoint(t *testing.T) {
	// Health endpoint should always return 200 OK
	mux := http.NewServeMux()
	Health(mux)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestReadyEndpoint_Success(t *testing.T) {
	// Ready endpoint should return 200 when Site CR can be fetched
	config := &internal.ServerConfig{
		SiteName:  "test-site",
		Namespace: "test-namespace",
	}

	mockClient := &mockSiteClient{
		site: &v1beta1.Site{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-site",
				Namespace: "test-namespace",
			},
		},
		err: nil,
	}

	mux := http.NewServeMux()
	Ready(mux, mockClient, config)

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Ready", w.Body.String())
}

func TestReadyEndpoint_Failure(t *testing.T) {
	// Ready endpoint should return 503 when Site CR cannot be fetched
	config := &internal.ServerConfig{
		SiteName:  "test-site",
		Namespace: "test-namespace",
	}

	mockClient := &mockSiteClient{
		site: nil,
		err:  errors.New("failed to connect to k8s API"),
	}

	mux := http.NewServeMux()
	Ready(mux, mockClient, config)

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "Not Ready", w.Body.String())
}
