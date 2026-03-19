// SPDX-FileCopyrightText: 2023, 2024, 2025 Markus Sommer
// SPDX-FileCopyrightText: 2023, 2024 Artur Sterz
//
// SPDX-License-Identifier: GPL-3.0-or-later

// Package processing implements the "Bundle Processing" steps from bpv7
// See: https://www.rfc-editor.org/rfc/rfc9171.html#name-bundle-processing
package processing

import (
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
	"github.com/dtn7/dtn7-go/pkg/routing"
	"github.com/dtn7/dtn7-go/pkg/store"
)

var ownNodeID bpv7.EndpointID

func SetOwnNodeID(nid bpv7.EndpointID) {
	ownNodeID = nid
}

// forwardingAsync implements the bundle forwarding procedure described in RFC9171 section 5.4
func forwardingAsync(bundleDescriptor *store.BundleDescriptor, bundle *bpv7.Bundle) {
	log.WithField("bundle", bundleDescriptor).Debug("Processing bundle")

	// Step 1: add "Forward Pending", remove "Dispatch Pending"
	err := bundleDescriptor.AddConstraint(store.ForwardPending)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": bundleDescriptor,
			"error":  err,
		}).Error("Error adding constraint to bundle")
		return
	}
	err = bundleDescriptor.RemoveConstraint(store.DispatchPending)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": bundleDescriptor,
			"error":  err,
		}).Error("Error removing constraint from bundle")
		return
	}

	// Step 2: determine if contraindicated - whatever that means
	// Step 2.1: Call routing algorithm(?)
	peers := cla.GetManagerSingleton().GetFilteredPeers(bundleDescriptor)

	// attempt direct delivery
	directDelivery(bundleDescriptor, bundle, peers)

	// Ask routing algorithm if we should send to anyone else
	forwardToPeers := routing.GetAlgorithmSingleton().SelectPeersForForwarding(bundleDescriptor, peers)

	// Step 3: if contraindicated, call `contraindicateBundle`, and return
	if len(forwardToPeers) == 0 {
		bundleContraindicated(bundleDescriptor)
		return
	}

	// Steps 4-6 happen in separate function
	forwardBundleToPeers(bundleDescriptor, bundle, forwardToPeers)
}

func BundleForwarding(bundleDescriptor *store.BundleDescriptor, bundle *bpv7.Bundle) {
	go forwardingAsync(bundleDescriptor, bundle)
}

func bundleContraindicated(bundleDescriptor *store.BundleDescriptor) {
	// TODO: is there anything else to do here?
	err := bundleDescriptor.ResetConstraints()
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": bundleDescriptor,
			"error":  err,
		}).Error("Error resetting bundle constraints")
	}
}

func directDelivery(bundleDescriptor *store.BundleDescriptor, bundle *bpv7.Bundle, peers []cla.ConvergenceSender) {
	metadata, err := bundleDescriptor.Metadata()
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": bundleDescriptor.ID(),
			"error":  err,
		}).Debug("Could not get bundle metadata")
		return
	}

	deliveries := make([]cla.ConvergenceSender, 0, len(peers))

	for _, peer := range peers {
		if peer.GetPeerEndpointID() == metadata.Destination {
			deliveries = append(deliveries, peer)
		}
	}

	if len(deliveries) > 0 {
		forwardBundleToPeers(bundleDescriptor, bundle, deliveries)
	}
}

func forwardBundleToPeers(bundleDescriptor *store.BundleDescriptor, bundle *bpv7.Bundle, peers []cla.ConvergenceSender) {
	var err error

	// load bundle if necessary
	if bundle == nil {
		bundle, err = bundleDescriptor.Load()
		if err != nil {
			log.WithFields(log.Fields{
				"bundle": bundleDescriptor,
				"error":  err,
			}).Error("Error loading bundle from disk")
			return
		}
	}

	// Step 4:
	// Step 4.1: remove previous node block
	if prevNodeBlock, err := bundle.ExtensionBlockByType(bpv7.BlockTypePreviousNodeBlock); err == nil {
		bundle.RemoveExtensionBlockByBlockNumber(prevNodeBlock.BlockNumber)
	}
	// Step 4.2: add new previous node block
	prevNodeBlock := bpv7.NewCanonicalBlock(0, 0, bpv7.NewPreviousNodeBlock(ownNodeID))
	err = bundle.AddExtensionBlock(prevNodeBlock)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": bundleDescriptor,
			"error":  err,
		}).Error("Error adding PreviousNodeBlock to bundle")
	}
	// TODO: Step 4.3: update bundle age block
	// Step 4.4: call CLAs for transmission
	var wg sync.WaitGroup
	wg.Add(len(peers))
	for _, peer := range peers {
		go forwardBundleToPeer(bundleDescriptor, bundle, peer, &wg)
	}
	wg.Wait()

	// Step 6: remove "Forward Pending"
	err = bundleDescriptor.RemoveConstraint(store.ForwardPending)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": bundleDescriptor,
			"error":  err,
		}).Error("Error removing constraint from bundle")
	}
}

func forwardBundleToPeer(bundleDescriptor *store.BundleDescriptor, bundle *bpv7.Bundle, peer cla.ConvergenceSender, wg *sync.WaitGroup) {
	log.WithFields(log.Fields{
		"bundle": bundle.ID(),
		"cla":    peer,
	}).Info("Sending bundle to a CLA (ConvergenceSender)")
	defer wg.Done()

	// allow routing algorithm to modify bundle
	// "extract" headers from bundle
	bundleHeaders := bpv7.BundleHeaders(bundle)
	err := routing.GetAlgorithmSingleton().ModifyHeaders(bundleDescriptor, bundleHeaders, peer)
	if err != nil {
		// if there was an error, we abort
		log.WithFields(log.Fields{
			"bundle": bundleDescriptor.ID(),
			"peer":   peer.GetPeerEndpointID(),
		})
		return
	}
	// "reconstruct" bundle from modified headers
	bundle = bpv7.BundleFromPartialBundle(bundleHeaders, bundle.PayloadBlock)

	if err := peer.Send(bundle); err != nil {
		log.WithFields(log.Fields{
			"bundle": bundle.ID(),
			"cla":    peer,
			"error":  err,
		}).Warn("Sending bundle failed")
	} else {
		log.WithFields(log.Fields{
			"bundle": bundle.ID(),
			"cla":    peer,
		}).Debug("Sending bundle succeeded")
		err = bundleDescriptor.AddKnownHolder(peer.GetPeerEndpointID())
		if err != nil {
			log.WithFields(log.Fields{
				"bundle": bundle.ID(),
				"error":  err,
			}).Debug("Error adding peer to known holders")
		}
	}
}

func DispatchPending() {
	log.Debug("Dispatching bundles")

	bundles := store.GetStoreSingleton().GetDispatchable()

	log.WithField("bundles", bundles).Debug("Bundles to dispatch")

	for _, bundle := range bundles {
		BundleForwarding(bundle, nil)
	}
}

func NewPeer(peerID bpv7.EndpointID) {
	routing.GetAlgorithmSingleton().NotifyPeerAppeared(peerID)
	DispatchPending()
}
