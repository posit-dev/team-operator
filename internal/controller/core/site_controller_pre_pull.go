package core

import (
	"context"
	"fmt"
	"strconv"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	"github.com/rstudio/goex/ptr"
	v12 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	v13 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime"
)

// deployPrePullDaemonset is a very simple prepull tool... it aims to pre-pull all images necessary for a site. It can
// easily be re-triggered by rolling the daemonset. We hope to improve this or use SOCCI at some point!
func deployPrePullDaemonset(ctx context.Context, r *SiteReconciler, req controllerruntime.Request, site *v1beta1.Site) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "deploy-prepull-daemonset",
	)

	containerList := product.ConcatLists(
		[]string{
			// TODO: we cannot prepull the home container because it does not have "echo"
			//   (i.e. it makes a bad init container...)
			//site.Spec.SiteHome.Image,
			site.Spec.Workbench.Image,
			site.Spec.Workbench.DefaultSessionImage,
			site.Spec.Connect.Image,
			site.Spec.Connect.SessionImage,
			site.Spec.Chronicle.Image,
			site.Spec.Chronicle.AgentImage,
			site.Spec.PackageManager.Image,
		},
		site.Spec.Workbench.ExtraSessionImages,
	)

	initContainerList := []v1.Container{}
	for i, container := range containerList {
		if container == "" {
			continue
		}
		initContainerList = append(initContainerList, v1.Container{
			Name:    "pre-pull-container-" + strconv.Itoa(i),
			Image:   container,
			Command: []string{"echo"},
			Args:    []string{"pre-pulling image: " + container},
		})
	}

	daemonsetName := fmt.Sprintf("%s-prepull", req.Name)

	prePullDaemonset := &v12.DaemonSet{
		ObjectMeta: v13.ObjectMeta{
			Name:      daemonsetName,
			Namespace: req.Namespace,
		},
	}

	if _, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, prePullDaemonset, site, func() error {
		prePullDaemonset.Labels = site.KubernetesLabels()
		prePullDaemonset.Spec = v12.DaemonSetSpec{
			Selector: &v13.LabelSelector{
				MatchLabels: map[string]string{
					// TODO: need to migrate this to something more formal...
					//   ... but it will require delete / re-create / etc.
					"app": daemonsetName,
				},
			},
			UpdateStrategy: v12.DaemonSetUpdateStrategy{
				Type: "RollingUpdate",
				RollingUpdate: &v12.RollingUpdateDaemonSet{
					MaxUnavailable: ptr.To(intstr.FromString("50%")),
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: v13.ObjectMeta{
					Labels: product.LabelMerge(
						site.KubernetesLabels(),
						map[string]string{
							"app": daemonsetName,
							// TODO: we can force rollover more often here if we want...
						}),
				},
				Spec: v1.PodSpec{
					EnableServiceLinks: ptr.To(false),
					InitContainers:     initContainerList,
					Containers: []v1.Container{
						{
							Name:  "sleep-forever",
							Image: "public.ecr.aws/docker/library/busybox",
							Command: []string{
								"sh",
								"-c",
								"while true; do echo 'images pulled. sleeping forever'; sleep infinity; done",
							},
						},
					},
				},
			},
		}

		if len(site.Spec.Workbench.Tolerations) > 0 {
			// add the tolerations to the daemonset
			for _, t := range site.Spec.Workbench.Tolerations {
				prePullDaemonset.Spec.Template.Spec.Tolerations = append(prePullDaemonset.Spec.Template.Spec.Tolerations, *t.DeepCopy())
			}

			// TODO: should also use the workbench node selectors...? But could differ from Connect...
		}
		return nil
	}); err != nil {
		l.Error(err, "error creating or updating pre-pull daemonset")
		return err
	}
	return nil
}
