// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package main

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/internal/observability"
	"github.com/posit-dev/team-operator/internal/status"
)

// multiKindLister implements observability.ResourceLister by listing all
// operator-managed CR kinds and returning per-(controller, namespace, phase) counts.
// It is wired into the async OTel gauge in main.go.
type multiKindLister struct {
	client client.Client
	log    logr.Logger
}

func (l *multiKindLister) List(ctx context.Context) ([]observability.ResourceCount, error) {
	var counts []observability.ResourceCount

	counts = append(counts, l.listSites(ctx)...)
	counts = append(counts, l.listConnects(ctx)...)
	counts = append(counts, l.listWorkbenches(ctx)...)
	counts = append(counts, l.listPackageManagers(ctx)...)
	counts = append(counts, l.listChronicles(ctx)...)
	counts = append(counts, l.listFlightdecks(ctx)...)
	counts = append(counts, l.listPostgresDatabases(ctx)...)

	return counts, nil
}

// readyPhase returns "ready" or "error" based on a boolean flag.
func readyPhase(ready bool) string {
	if ready {
		return observability.PhaseReady
	}
	return observability.PhaseError
}

func (l *multiKindLister) listSites(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.SiteList
	if err := l.client.List(ctx, &list); err != nil {
		l.log.V(1).Info("resource_count: list failed", "kind", "site", "err", err.Error())
		return nil
	}
	counts := make(map[[2]string]int64, len(list.Items))
	for i := range list.Items {
		phase := readyPhase(status.IsReady(list.Items[i].Status.Conditions))
		counts[[2]string{list.Items[i].Namespace, phase}]++
	}
	return mapToResourceCounts("site", counts)
}

func (l *multiKindLister) listConnects(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.ConnectList
	if err := l.client.List(ctx, &list); err != nil {
		l.log.V(1).Info("resource_count: list failed", "kind", "connect", "err", err.Error())
		return nil
	}
	counts := make(map[[2]string]int64, len(list.Items))
	for i := range list.Items {
		counts[[2]string{list.Items[i].Namespace, readyPhase(list.Items[i].Status.Ready)}]++
	}
	return mapToResourceCounts("connect", counts)
}

func (l *multiKindLister) listWorkbenches(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.WorkbenchList
	if err := l.client.List(ctx, &list); err != nil {
		l.log.V(1).Info("resource_count: list failed", "kind", "workbench", "err", err.Error())
		return nil
	}
	counts := make(map[[2]string]int64, len(list.Items))
	for i := range list.Items {
		counts[[2]string{list.Items[i].Namespace, readyPhase(list.Items[i].Status.Ready)}]++
	}
	return mapToResourceCounts("workbench", counts)
}

func (l *multiKindLister) listPackageManagers(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.PackageManagerList
	if err := l.client.List(ctx, &list); err != nil {
		l.log.V(1).Info("resource_count: list failed", "kind", "package-manager", "err", err.Error())
		return nil
	}
	counts := make(map[[2]string]int64, len(list.Items))
	for i := range list.Items {
		counts[[2]string{list.Items[i].Namespace, readyPhase(list.Items[i].Status.Ready)}]++
	}
	return mapToResourceCounts("package-manager", counts)
}

func (l *multiKindLister) listChronicles(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.ChronicleList
	if err := l.client.List(ctx, &list); err != nil {
		l.log.V(1).Info("resource_count: list failed", "kind", "chronicle", "err", err.Error())
		return nil
	}
	counts := make(map[[2]string]int64, len(list.Items))
	for i := range list.Items {
		counts[[2]string{list.Items[i].Namespace, readyPhase(list.Items[i].Status.Ready)}]++
	}
	return mapToResourceCounts("chronicle", counts)
}

func (l *multiKindLister) listFlightdecks(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.FlightdeckList
	if err := l.client.List(ctx, &list); err != nil {
		l.log.V(1).Info("resource_count: list failed", "kind", "flightdeck", "err", err.Error())
		return nil
	}
	counts := make(map[[2]string]int64, len(list.Items))
	for i := range list.Items {
		counts[[2]string{list.Items[i].Namespace, readyPhase(list.Items[i].Status.Ready)}]++
	}
	return mapToResourceCounts("flightdeck", counts)
}

func (l *multiKindLister) listPostgresDatabases(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.PostgresDatabaseList
	if err := l.client.List(ctx, &list); err != nil {
		l.log.V(1).Info("resource_count: list failed", "kind", "postgres-database", "err", err.Error())
		return nil
	}
	counts := make(map[[2]string]int64, len(list.Items))
	for i := range list.Items {
		// PostgresDatabaseStatus embeds CommonProductStatus (Conditions) but has no
		// direct Ready bool field; use status.IsReady on the Conditions slice.
		phase := readyPhase(status.IsReady(list.Items[i].Status.Conditions))
		counts[[2]string{list.Items[i].Namespace, phase}]++
	}
	return mapToResourceCounts("postgres-database", counts)
}

// mapToResourceCounts converts a namespace/phase count map into ResourceCount observations.
func mapToResourceCounts(controller string, m map[[2]string]int64) []observability.ResourceCount {
	out := make([]observability.ResourceCount, 0, len(m))
	for k, n := range m {
		out = append(out, observability.ResourceCount{
			Controller: controller,
			Namespace:  k[0],
			Phase:      k[1],
			Count:      n,
		})
	}
	return out
}
