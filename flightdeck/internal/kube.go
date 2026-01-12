package internal

import (
	"context"
	"fmt"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log/slog"
	"os"
)

type KubeClient struct {
	Config     *rest.Config
	Client     *rest.RESTClient
	SiteClient SiteInterface
}

var SchemeGroupVersion = schema.GroupVersion{
	Group:   positcov1beta1.SchemeGroupVersion.Group,
	Version: positcov1beta1.SchemeGroupVersion.Version}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&positcov1beta1.Site{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

func getKubeConfigPath() string {
	k, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
		k = fmt.Sprintf("%s/.kube/config", os.Getenv("HOME"))
	}
	return k
}

func NewKubeClient() (*KubeClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeConfigPath := getKubeConfigPath()
		slog.Warn("Unable to get in-cluster config, fetching from kubeconfig", "error", err, "kubeconfig", kubeConfigPath)
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return nil, err
		}
	}

	err = AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}

	crdConfig := *config
	crdConfig.ContentConfig.GroupVersion = &schema.GroupVersion{
		Group:   positcov1beta1.SchemeGroupVersion.Group,
		Version: positcov1beta1.SchemeGroupVersion.Version}
	crdConfig.APIPath = "/apis"
	crdConfig.NegotiatedSerializer = serializer.NewCodecFactory(scheme.Scheme)
	crdConfig.UserAgent = rest.DefaultKubernetesUserAgent()

	c, err := rest.UnversionedRESTClientFor(&crdConfig)
	if err != nil {
		panic(err)
	}

	s := &siteClient{
		restClient: c,
	}

	return &KubeClient{
		Config:     config,
		Client:     c,
		SiteClient: s,
	}, nil
}

type SiteInterface interface {
	Get(name string, namespace string, options metav1.GetOptions, ctx context.Context) (*positcov1beta1.Site, error)
}

type siteClient struct {
	restClient rest.Interface
	ns         string
}

func (c *siteClient) Get(name string, namespace string, opts metav1.GetOptions, ctx context.Context) (*positcov1beta1.Site, error) {
	result := positcov1beta1.Site{}
	slog.Debug("fetching site from kubernetes", "name", name, "namespace", namespace)
	err := c.restClient.
		Get().
		Namespace(namespace).
		Resource("sites").
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(&result)

	if err != nil {
		slog.Error("failed to fetch site", "name", name, "namespace", namespace, "error", err)
		return &result, err
	}

	slog.Debug("site fetched successfully", "name", name, "namespace", namespace)
	return &result, err
}
