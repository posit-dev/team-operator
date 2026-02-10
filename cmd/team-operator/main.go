// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package main

import (
	"flag"
	"os"
	"strconv"

	"github.com/posit-dev/team-operator/api/keycloak/v2alpha1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/internal"

	corecontroller "github.com/posit-dev/team-operator/internal/controller/core"

	//+kubebuilder:scaffold:imports

	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup").V(determineLogLevel("TEAM_OPERATOR_LOG_LEVEL"))
)

func determineLogLevel(envVar string) int {
	logLevel := os.Getenv(envVar)
	if logLevel == "" {
		return 0
	}
	level, err := strconv.Atoi(logLevel)
	if err != nil {
		return 0
	}
	return level
}

func LoadSchemes(scheme *runtime.Scheme) {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(positcov1beta1.AddToScheme(scheme))

	// IMPORTANT: register schemes for other CRDs we need to create
	// secret store
	utilruntime.Must(secretsstorev1.AddToScheme(scheme))
	// traefik
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	// keycloak
	utilruntime.Must(v2alpha1.AddToScheme(scheme))
}

func init() {
	//+kubebuilder:scaffold:scheme
	LoadSchemes(scheme)
}

func main() {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")

	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for team-operator. "+
			"Enabling this will ensure there is only one active team-operator.")

	opts := zap.Options{Development: true}

	opts.BindFlags(flag.CommandLine)
	klog.InitFlags(flag.CommandLine)

	flag.Parse()

	zl := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(zl)
	klog.SetLogger(zl)

	zl.Info("team-operator version", "version", internal.VersionString)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "fd877d62.posit.co",
		LeaderElectionNamespace: product.PositTeamSystemNamespace,
		Cache: cache.Options{
			DefaultNamespaces: internal.GetWatchNamespaces(),
		},
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// team-operator stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after team-operator stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start team-operator")
		os.Exit(1)
	}

	if err = (&corecontroller.SiteReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    setupLog,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Site")
		os.Exit(1)
	}

	if err = (&corecontroller.PostgresDatabaseReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    setupLog,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PostgresDatabase")
		os.Exit(1)
	}

	if err = (&corecontroller.ConnectReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    setupLog,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ImplConnect")
		os.Exit(1)
	}

	if err = (&corecontroller.WorkbenchReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Workbench")
		os.Exit(1)
	}

	if err = (&corecontroller.PackageManagerReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    setupLog,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PackageManager")
		os.Exit(1)
	}

	if err = (&corecontroller.ChronicleReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    setupLog,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Chronicle")
		os.Exit(1)
	}

	if err = (&corecontroller.FlightdeckReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    setupLog,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Flightdeck")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting team-operator")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running team-operator")
		os.Exit(1)
	}
}
