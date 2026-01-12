package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTraefikConstant(t *testing.T) {
	// copied from traefik website...
	// https://doc.traefik.io/traefik/routing/providers/kubernetes-ingress/#annotations
	assert.Equal(t, "traefik.ingress.kubernetes.io/router.middlewares", TraefikMiddlewaresKey)
}

func TestBuildTraefikMiddlewareAnnotation(t *testing.T) {
	res := BuildTraefikMiddlewareAnnotation("ns", "middleware1", "middleware2")
	assert.Equal(t, "ns-middleware1@kubernetescrd,ns-middleware2@kubernetescrd", res)
}
