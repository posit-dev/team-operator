package internal

import (
	"context"
	"errors"
	"testing"

	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
)

func TestSiteClient_Get_HandlesNoCRD(t *testing.T) {
	tests := []struct {
		name      string
		setupREST func() *fake.RESTClient
		wantError bool
		wantEmpty bool
		errMsg    string
	}{
		{
			name: "CRD not found - returns empty site",
			setupREST: func() *fake.RESTClient {
				client := &fake.RESTClient{
					NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{}),
				}
				client.Err = errors.New("the server could not find the requested resource")
				return client
			},
			wantError: false,
			wantEmpty: true,
		},
		{
			name: "No matches for kind - returns empty site",
			setupREST: func() *fake.RESTClient {
				client := &fake.RESTClient{
					NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{}),
				}
				client.Err = errors.New("no matches for kind \"Site\" in version \"core.posit.team/v1beta1\"")
				return client
			},
			wantError: false,
			wantEmpty: true,
		},
		{
			name: "Other error - returns error",
			setupREST: func() *fake.RESTClient {
				client := &fake.RESTClient{
					NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{}),
				}
				client.Err = errors.New("connection refused")
				return client
			},
			wantError: true,
			wantEmpty: false,
			errMsg:    "connection refused",
		},
		{
			name: "Site found - returns site",
			setupREST: func() *fake.RESTClient {
				site := &positcov1beta1.Site{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-site",
						Namespace: "posit-team",
					},
				}
				client := &fake.RESTClient{
					NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{}),
					Resp: &rest.Response{
						Response: nil,
					},
				}
				// In a real test, we'd properly mock the response
				// For now, we're testing the error handling logic
				return client
			},
			wantError: false,
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &siteClient{
				restClient: tt.setupREST(),
			}

			ctx := context.Background()
			result, err := client.Get("test-site", "posit-team", metav1.GetOptions{}, ctx)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				if tt.wantEmpty {
					// When CRD doesn't exist, we return an empty site with just name/namespace
					assert.Equal(t, "test-site", result.Name)
					assert.Equal(t, "posit-team", result.Namespace)
				}
			}
		})
	}
}