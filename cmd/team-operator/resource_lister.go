// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package main

import (
	"context"

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

// tally aggregates a slice of (namespace, phase) pairs into ResourceCount observations.
func tally(controller string, observations []struct{ ns, phase string }) []observability.ResourceCount {
	type key struct{ ns, phase string }
	m := map[key]int64{}
	for _, o := range observations {
		m[key{o.ns, o.phase}]++
	}
	out := make([]observability.ResourceCount, 0, len(m))
	for k, n := range m {
		out = append(out, observability.ResourceCount{
			Controller: controller,
			Namespace:  k.ns,
			Phase:      k.phase,
			Count:      n,
		})
	}
	return out
}

func (l *multiKindLister) listSites(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.SiteList
	if err := l.client.List(ctx, &list); err != nil {
		return nil
	}
	obs := make([]struct{ ns, phase string }, 0, len(list.Items))
	for _, cr := range list.Items {
		// Site has no direct Ready bool; derive readiness from Conditions.
		obs = append(obs, struct{ ns, phase string }{
			ns:    cr.Namespace,
			phase: readyPhase(status.IsReady(cr.Status.Conditions)),
		})
	}
	return tally("site", obs)
}

func (l *multiKindLister) listConnects(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.ConnectList
	if err := l.client.List(ctx, &list); err != nil {
		return nil
	}
	obs := make([]struct{ ns, phase string }, 0, len(list.Items))
	for _, cr := range list.Items {
		obs = append(obs, struct{ ns, phase string }{
			ns:    cr.Namespace,
			phase: readyPhase(cr.Status.Ready),
		})
	}
	return tally("connect", obs)
}

func (l *multiKindLister) listWorkbenches(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.WorkbenchList
	if err := l.client.List(ctx, &list); err != nil {
		return nil
	}
	obs := make([]struct{ ns, phase string }, 0, len(list.Items))
	for _, cr := range list.Items {
		obs = append(obs, struct{ ns, phase string }{
			ns:    cr.Namespace,
			phase: readyPhase(cr.Status.Ready),
		})
	}
	return tally("workbench", obs)
}

func (l *multiKindLister) listPackageManagers(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.PackageManagerList
	if err := l.client.List(ctx, &list); err != nil {
		return nil
	}
	obs := make([]struct{ ns, phase string }, 0, len(list.Items))
	for _, cr := range list.Items {
		obs = append(obs, struct{ ns, phase string }{
			ns:    cr.Namespace,
			phase: readyPhase(cr.Status.Ready),
		})
	}
	return tally("package-manager", obs)
}

func (l *multiKindLister) listChronicles(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.ChronicleList
	if err := l.client.List(ctx, &list); err != nil {
		return nil
	}
	obs := make([]struct{ ns, phase string }, 0, len(list.Items))
	for _, cr := range list.Items {
		obs = append(obs, struct{ ns, phase string }{
			ns:    cr.Namespace,
			phase: readyPhase(cr.Status.Ready),
		})
	}
	return tally("chronicle", obs)
}

func (l *multiKindLister) listFlightdecks(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.FlightdeckList
	if err := l.client.List(ctx, &list); err != nil {
		return nil
	}
	obs := make([]struct{ ns, phase string }, 0, len(list.Items))
	for _, cr := range list.Items {
		obs = append(obs, struct{ ns, phase string }{
			ns:    cr.Namespace,
			phase: readyPhase(cr.Status.Ready),
		})
	}
	return tally("flightdeck", obs)
}

func (l *multiKindLister) listPostgresDatabases(ctx context.Context) []observability.ResourceCount {
	var list positcov1beta1.PostgresDatabaseList
	if err := l.client.List(ctx, &list); err != nil {
		return nil
	}
	obs := make([]struct{ ns, phase string }, 0, len(list.Items))
	for _, cr := range list.Items {
		// PostgresDatabaseStatus embeds CommonProductStatus (Conditions) but has no
		// direct Ready bool field; use status.IsReady on the Conditions slice.
		obs = append(obs, struct{ ns, phase string }{
			ns:    cr.Namespace,
			phase: readyPhase(status.IsReady(cr.Status.Conditions)),
		})
	}
	return tally("postgres-database", obs)
}
