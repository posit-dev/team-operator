package core

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/internal"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errNetworkPolicyCleanup = errors.New("network policy cleanup error")
)

const (
	grafanaAlloyNamespace = "alloy"
)

func (r *SiteReconciler) reconcileNetworkPolicies(ctx context.Context, req ctrl.Request, site *v1beta1.Site) error {
	l := r.GetLogger(ctx).WithValues("event", "reconcile-networkpolicies")

	l = l.WithValues("network_trust", site.Spec.NetworkTrust)

	if site.Spec.NetworkTrust > v1beta1.NetworkTrustSameSite {
		l.Info("network trust does not require policies")

		if err := r.cleanupNetworkPolicies(ctx, req); err != nil {
			l.Error(err, "failed to clean up policies")
		}

		return nil
	}

	if err := r.reconcileChronicleNetworkPolicy(ctx, req.Namespace, l, site); err != nil {
		l.Error(err, "error ensuring chronicle network policy")
		return err
	}

	if err := r.reconcileConnectNetworkPolicy(ctx, req.Namespace, l, site); err != nil {
		l.Error(err, "error ensuring connect network policy")
		return err
	}

	if err := r.reconcileConnectSessionNetworkPolicy(ctx, req.Namespace, l, site); err != nil {
		l.Error(err, "error ensuring connect session network policy")
		return err
	}

	if err := r.reconcileHomeNetworkPolicy(ctx, req.Namespace, l, site); err != nil {
		l.Error(err, "error ensuring home network policy")
		return err
	}

	if err := r.reconcileKeycloakNetworkPolicy(ctx, req.Namespace, l, site); err != nil {
		l.Error(err, "error ensuring keycloak network policy")
		return err
	}

	if err := r.reconcilePackageManagerNetworkPolicy(ctx, req.Namespace, l, site); err != nil {
		l.Error(err, "error ensuring package manager network policy")
		return err
	}

	if err := r.reconcileWorkbenchNetworkPolicy(ctx, req.Namespace, l, site); err != nil {
		l.Error(err, "error ensuring workbench network policy")
		return err
	}

	if err := r.reconcileWorkbenchSessionNetworkPolicy(ctx, req.Namespace, l, site); err != nil {
		l.Error(err, "error ensuring workbench session network policy")
		return err
	}

	return nil
}

func (r *SiteReconciler) cleanupNetworkPolicies(ctx context.Context, req ctrl.Request) error {
	l := r.GetLogger(ctx).WithValues("event", "cleanup-networkpolicies")

	var cleanupErr error

	for _, component := range []string{
		"chronicle",
		"connect",
		"connect-session",
		"home",
		"keycloak",
		"packagemanager",
		"workbench",
		"workbench-session",
	} {
		key := client.ObjectKey{
			Name:      req.Name + "-" + component,
			Namespace: req.Namespace,
		}

		if err := internal.BasicDelete(ctx, r, l, key, &networkingv1.NetworkPolicy{}); err != nil {
			l.Error(err, "error cleaning up network policy", "key", key)

			if cleanupErr == nil {
				cleanupErr = errors.Wrapf(errNetworkPolicyCleanup, "%v", key.Name)
			} else {
				cleanupErr = errors.Wrapf(cleanupErr, "%v", key.Name)
			}
		}
	}

	return cleanupErr
}

func (r *SiteReconciler) reconcileChronicleNetworkPolicy(ctx context.Context, namespace string, l logr.Logger, site *v1beta1.Site) error {
	policyName := site.Name + "-chronicle"

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespace,
		},
	}
	_, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, policy, site, func() error {
		policy.Labels = site.KubernetesLabels()
		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1.SiteLabelKey:               site.Name,
					v1beta1.KubernetesInstanceLabelKey: policyName,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
				networkingv1.PolicyTypeIngress,
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey: site.Name,
								},
							},
						},
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.KubernetesMetadataNameKey: grafanaAlloyNamespace,
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						internal.DefaultPortChronicleHTTP.NetworkPolicyPort(),
						internal.DefaultPortChronicleMetrics.NetworkPolicyPort(),
					},
				},
			},
		}
		return nil
	})
	return err
}

func (r *SiteReconciler) reconcileConnectNetworkPolicy(ctx context.Context, namespace string, l logr.Logger, site *v1beta1.Site) error {
	policyName := site.Name + "-connect"

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespace,
		},
	}
	_, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, policy, site, func() error {
		policy.Labels = site.KubernetesLabels()
		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1.SiteLabelKey:               site.Name,
					v1beta1.KubernetesInstanceLabelKey: policyName,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
				networkingv1.PolicyTypeIngress,
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								// TODO: look up CIDR block that contains all private addresses
								CIDR: "10.0.0.0/8",
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						internal.DefaultPortHTTPS.NetworkPolicyPort(),
						internal.DefaultPortPostgres.NetworkPolicyPort(),
					},
				},
				internal.PublicInternetNetworkPolicyEgressRule(),
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey:      site.Name,
									v1beta1.ComponentLabelKey: "workbench",
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey:      site.Name,
									v1beta1.ComponentLabelKey: "connect",
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey:          site.Name,
									v1beta1.LauncherInstanceIDKey: site.Name + "-workbench",
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey:          site.Name,
									v1beta1.LauncherInstanceIDKey: site.Name + "-connect",
								},
							},
						},
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.KubernetesMetadataNameKey: "traefik",
								},
							},
						},
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.KubernetesMetadataNameKey: grafanaAlloyNamespace,
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						internal.DefaultPortConnectHTTP.NetworkPolicyPort(),
						internal.DefaultPortConnectMetrics.NetworkPolicyPort(),
						internal.DefaultPortLauncher.NetworkPolicyPort(),
					},
				},
			},
		}
		return nil
	})
	return err
}

func (r *SiteReconciler) reconcileConnectSessionNetworkPolicy(ctx context.Context, namespace string, l logr.Logger, site *v1beta1.Site) error {
	policyName := site.Name + "-connect-session"

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespace,
		},
	}
	_, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, policy, site, func() error {
		policy.Labels = site.KubernetesLabels()
		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1.SiteLabelKey:      site.Name,
					v1beta1.ComponentLabelKey: v1beta1.ComponentLabelValueConnectSession,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
				networkingv1.PolicyTypeIngress,
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				// allow only outbound internet access
				internal.PublicInternetOnlyNetworkPolicyEgressRule(),
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							// egress to parent Connect
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey:      site.Name,
									v1beta1.ComponentLabelKey: v1beta1.ComponentLabelValueConnect,
								},
							},
						},
					},
				},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							// ingress from parent Connect
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey:      site.Name,
									v1beta1.ComponentLabelKey: v1beta1.ComponentLabelValueConnect,
								},
							},
						},
						{
							// ingress from grafana agent
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.KubernetesMetadataNameKey: grafanaAlloyNamespace,
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						internal.DefaultPortConnectHTTP.NetworkPolicyPort(),
						internal.DefaultPortConnectSession.NetworkPolicyPort(),
					},
				},
			},
		}
		return nil
	})
	return err
}

func (r *SiteReconciler) reconcileHomeNetworkPolicy(ctx context.Context, namespace string, l logr.Logger, site *v1beta1.Site) error {
	policyName := site.Name + "-home"

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespace,
		},
	}
	_, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, policy, site, func() error {
		policy.Labels = site.KubernetesLabels()
		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1.SiteLabelKey:               site.Name,
					v1beta1.KubernetesInstanceLabelKey: policyName,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
				networkingv1.PolicyTypeIngress,
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.KubernetesMetadataNameKey: "traefik",
								},
							},
						},
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.KubernetesMetadataNameKey: grafanaAlloyNamespace,
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						internal.DefaultPortHomeHTTP.NetworkPolicyPort(),
					},
				},
			},
		}
		return nil
	})
	return err
}

func (r *SiteReconciler) reconcileKeycloakNetworkPolicy(ctx context.Context, namespace string, l logr.Logger, site *v1beta1.Site) error {
	policyName := site.Name + "-keycloak"

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespace,
		},
	}
	_, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, policy, site, func() error {
		policy.Labels = site.KubernetesLabels()
		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                              "keycloak",
					v1beta1.KubernetesInstanceLabelKey: site.Name,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
				networkingv1.PolicyTypeIngress,
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								// TODO: look up CIDR block that contains all private addresses
								CIDR: "10.0.0.0/8",
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						internal.DefaultPortPostgres.NetworkPolicyPort(),
					},
				},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.KubernetesMetadataNameKey: "traefik",
								},
							},
						},
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.KubernetesMetadataNameKey: grafanaAlloyNamespace,
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						internal.DefaultPortKeycloakHTTP.NetworkPolicyPort(),
						internal.DefaultPortKeycloakHTTPS.NetworkPolicyPort(),
					},
				},
			},
		}
		return nil
	})
	return err
}

func (r *SiteReconciler) reconcilePackageManagerNetworkPolicy(ctx context.Context, namespace string, l logr.Logger, site *v1beta1.Site) error {
	policyName := site.Name + "-packagemanager"

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespace,
		},
	}
	_, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, policy, site, func() error {
		policy.Labels = site.KubernetesLabels()
		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1.SiteLabelKey:               site.Name,
					v1beta1.KubernetesInstanceLabelKey: policyName,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
				networkingv1.PolicyTypeIngress,
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey:               site.Name,
									v1beta1.KubernetesInstanceLabelKey: site.Name + "-workbench",
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey:          site.Name,
									v1beta1.LauncherInstanceIDKey: site.Name + "-workbench",
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey:               site.Name,
									v1beta1.KubernetesInstanceLabelKey: site.Name + "-connect",
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey:               site.Name,
									v1beta1.KubernetesInstanceLabelKey: site.Name + "-connect",
								},
							},
						},
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.KubernetesMetadataNameKey: "traefik",
								},
							},
						},
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.KubernetesMetadataNameKey: grafanaAlloyNamespace,
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						internal.DefaultPortPackageManagerHTTP.NetworkPolicyPort(),
						internal.DefaultPortPackageManagerMetrics.NetworkPolicyPort(),
					},
				},
			},
		}
		return nil
	})
	return err
}

func (r *SiteReconciler) reconcileWorkbenchNetworkPolicy(ctx context.Context, namespace string, l logr.Logger, site *v1beta1.Site) error {
	policyName := site.Name + "-workbench"

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespace,
		},
	}
	_, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, policy, site, func() error {
		policy.Labels = site.KubernetesLabels()
		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1.SiteLabelKey:      site.Name,
					v1beta1.ComponentLabelKey: v1beta1.ComponentLabelValueWorkbench,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
				networkingv1.PolicyTypeIngress,
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							// from other workbenches
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey:      site.Name,
									v1beta1.ComponentLabelKey: v1beta1.ComponentLabelValueWorkbench,
								},
							},
						},
						{
							// from workbench sessions
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey:      site.Name,
									v1beta1.ComponentLabelKey: v1beta1.ComponentLabelValueWorkbenchSession,
								},
							},
						},
						{
							// from traefik
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.KubernetesMetadataNameKey: "traefik",
								},
							},
						},
						{
							// from grafana agent
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.KubernetesMetadataNameKey: grafanaAlloyNamespace,
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						internal.DefaultPortLauncher.NetworkPolicyPort(),
						internal.DefaultPortWorkbenchHTTP.NetworkPolicyPort(),
						internal.DefaultPortWorkbenchMetrics.NetworkPolicyPort(),
					},
				},
			},
		}
		return nil
	})
	return err
}

func (r *SiteReconciler) reconcileWorkbenchSessionNetworkPolicy(ctx context.Context, namespace string, l logr.Logger, site *v1beta1.Site) error {
	policyName := site.Name + "-workbench-session"

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespace,
		},
	}
	_, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, policy, site, func() error {
		// Build egress rules
		egressRules := []networkingv1.NetworkPolicyEgressRule{
			{
				To: []networkingv1.NetworkPolicyPeer{
					{
						// to parent workbench host
						PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1.SiteLabelKey:      site.Name,
								v1beta1.ComponentLabelKey: v1beta1.ComponentLabelValueWorkbench,
							},
						},
					},
				},
			},
			// access to the internet with no private networks
			internal.PublicInternetOnlyNetworkPolicyEgressRule(),
		}

		// Add EFS egress rule if enabled
		if site.Spec.EFSEnabled && site.Spec.VPCCIDR != "" {
			tcp := v1.ProtocolTCP
			port2049 := intstr.FromInt(2049)
			egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{
					{
						IPBlock: &networkingv1.IPBlock{
							CIDR: site.Spec.VPCCIDR,
						},
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: &tcp,
						Port:     &port2049,
					},
				},
			})
		}

		policy.Labels = site.KubernetesLabels()
		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1.SiteLabelKey:      site.Name,
					v1beta1.ComponentLabelKey: v1beta1.ComponentLabelValueWorkbenchSession,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
				networkingv1.PolicyTypeIngress,
			},
			Egress: egressRules,
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							// from workbench host
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.SiteLabelKey:      site.Name,
									v1beta1.ComponentLabelKey: v1beta1.ComponentLabelValueWorkbench,
								},
							},
						},
						{
							// from grafana agent
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1.KubernetesMetadataNameKey: grafanaAlloyNamespace,
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						internal.DefaultPortWorkbenchSessionHTTP.NetworkPolicyPort(),
						internal.DefaultPortWorkbenchSessionProxy.NetworkPolicyPort(),
					},
				},
			},
		}
		return nil
	})
	return err
}
