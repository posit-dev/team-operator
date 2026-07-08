// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package observability

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ResourceCount holds one gauge observation: how many CRs of a given controller
// are in a given namespace and phase.
type ResourceCount struct {
	Controller string
	Namespace  string
	Phase      string
	Count      int64
}

// ResourceLister is implemented by types that can list CRs of all kinds and
// return per-(controller, namespace, phase) counts.
type ResourceLister interface {
	List(ctx context.Context) ([]ResourceCount, error)
}

// RegisterResourceCountGauge registers an async gauge on m that calls lister.List
// on each OTel collection cycle.
func RegisterResourceCountGauge(m metric.Meter, lister ResourceLister) error {
	_, err := m.Int64ObservableGauge(
		MetricResourceCount,
		metric.WithDescription("Number of operator-managed CRs, partitioned by controller, namespace, and phase."),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			counts, err := lister.List(ctx)
			if err != nil {
				return nil
			}
			for _, c := range counts {
				o.Observe(c.Count,
					metric.WithAttributes(
						attribute.String(LabelController, c.Controller),
						attribute.String(LabelNamespace, c.Namespace),
						attribute.String(LabelPhase, c.Phase),
					),
				)
			}
			return nil
		}),
	)
	return err
}
