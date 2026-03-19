// SPDX-FileCopyrightText: 2019, 2022, 2026 Markus Sommer
// SPDX-FileCopyrightText: 2019, 2020 Alvar Penning
//
// SPDX-License-Identifier: GPL-3.0-or-later

package routing

import (
	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
	"github.com/dtn7/dtn7-go/pkg/store"
)

// EpidemicRouting is an implementation of an Algorithm and behaves in a
// flooding-based epidemic way.
type EpidemicRouting struct{}

// NewEpidemicRouting creates a new EpidemicRouting Algorithm interacting
// with the given Core.
func NewEpidemicRouting() *EpidemicRouting {
	log.Debug("Initialised epidemic routing")

	return &EpidemicRouting{}
}

// NotifyNewBundle does nothing for this algorithm
func (er *EpidemicRouting) NotifyNewBundle(_ *store.BundleDescriptor, _ *bpv7.Bundle) {}

// NotifyReceivedBundle does nothing for this algorithm
func (er *EpidemicRouting) NotifyReceivedBundle(_ *store.BundleDescriptor, _ *bpv7.Bundle) {}

// NotifyReceivedAdministrativeRecord does nothing for this algorithm
func (er *EpidemicRouting) NotifyReceivedAdministrativeRecord(_ *bpv7.Bundle) bool {
	return true
}

func (er *EpidemicRouting) SelectPeersForForwarding(_ *store.BundleDescriptor, peers []cla.ConvergenceSender) []cla.ConvergenceSender {
	return peers
}

// ModifyHeaders does nothing for this algorithm
func (er *EpidemicRouting) ModifyHeaders(_ *store.BundleDescriptor, _ *bpv7.PartialBundle, _ cla.ConvergenceSender) error {
	return nil
}

func (_ *EpidemicRouting) NotifyPeerAppeared(_ bpv7.EndpointID) {}

func (_ *EpidemicRouting) NotifyPeerDisappeared(_ bpv7.EndpointID) {}
