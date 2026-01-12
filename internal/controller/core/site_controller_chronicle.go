package core

import (
	"context"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *SiteReconciler) reconcileChronicle(ctx context.Context, req controllerruntime.Request, site *v1beta1.Site) error {

	l := r.GetLogger(ctx).WithValues(
		"event", "reconcile-chronicle",
	)

	chronicleLogLevel := v1beta1.ChronicleServiceLogLevelInfo
	if site.Spec.Debug {
		chronicleLogLevel = v1beta1.ChronicleServiceLogLevelDebug
	}

	chronicleLogFormat := v1beta1.ChronicleServiceLogFormatText

	if site.Spec.LogFormat == product.LogFormatJson {
		chronicleLogFormat = v1beta1.ChronicleServiceLogFormatJson
	}

	chronicleServerImage := site.Spec.Chronicle.Image

	targetChronicle := &v1beta1.Chronicle{
		ObjectMeta: v1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
			Labels: map[string]string{
				v1beta1.ManagedByLabelKey: LabelManagedByValue,
			},
			OwnerReferences: site.OwnerReferencesForChildren(),
		},
		Spec: v1beta1.ChronicleSpec{
			AwsAccountId:         site.Spec.AwsAccountId,
			ClusterDate:          site.Spec.ClusterDate,
			WorkloadCompoundName: site.Spec.WorkloadCompoundName,
			Config: v1beta1.ChronicleConfig{
				Http: &v1beta1.ChronicleHttpConfig{
					Listen: ":5252",
				},
				Logging: &v1beta1.ChronicleLoggingConfig{
					ServiceLog:       "STDOUT",
					ServiceLogLevel:  chronicleLogLevel,
					ServiceLogFormat: chronicleLogFormat,
				},
				Metrics: &v1beta1.ChronicleMetricsConfig{
					Enabled: true,
				},
				Profiling: &v1beta1.ChronicleProfilingConfig{
					Enabled: false,
					Listen:  ":3030",
				},
				// -- NOTE: the storage settings are set below
				S3Storage:    nil,
				LocalStorage: nil,
				// ^^ NOTE: the storage settings are set below
			},
			ImagePullSecrets: site.Spec.ImagePullSecrets,
			NodeSelector:     site.Spec.Chronicle.NodeSelector,
			AddEnv:           site.Spec.Chronicle.AddEnv,
			Image:            chronicleServerImage,
		},
	}

	// configure storage mechanism based on whether s3 bucket is set...
	if site.Spec.Chronicle.S3Bucket == "" {
		targetChronicle.Spec.Config.LocalStorage = &v1beta1.ChronicleLocalStorageConfig{
			Enabled:  true,
			Location: "/var/lib/posit-chronicle/data",
		}
	} else {
		// using s3 storage...
		targetChronicle.Spec.Config.S3Storage = &v1beta1.ChronicleS3StorageConfig{
			Enabled: true,
			Bucket:  site.Spec.Chronicle.S3Bucket,
			Prefix:  site.Name + "/chr-v0",
			// TODO: should not be hard-coded
			Region: "us-east-2",
		}
	}

	existingChronicle := v1beta1.Chronicle{}
	chrKey := client.ObjectKey{Name: req.Name, Namespace: req.Namespace}

	if err := internal.BasicCreateOrUpdate(ctx, r, l, chrKey, &existingChronicle, targetChronicle); err != nil {
		l.Error(err, "error creating chronicle")
		return err
	}
	return nil
}
