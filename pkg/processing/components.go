// SPDX-FileCopyrightText: 2026 Markus Sommer
//
// SPDX-License-Identifier: GPL-3.0-or-later

package processing

import (
	"github.com/go-co-op/gocron/v2"

	"github.com/dtn7/dtn7-go/pkg/application_agent"
	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
	"github.com/dtn7/dtn7-go/pkg/routing"
	"github.com/dtn7/dtn7-go/pkg/store"
)

// Components are all the components from other parts of the program,
// that the processing pipeline needs to do its job.
type Components struct {
	// This node's administrative endpoint
	NodeID    bpv7.EndpointID
	Scheduler gocron.Scheduler

	Store            *store.BundleStore
	CLAManager       *cla.Manager
	AAManager        *application_agent.Manager
	RoutingAlgorithm routing.Algorithm
}
