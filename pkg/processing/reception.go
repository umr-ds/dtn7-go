package processing

import (
	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/store"
)

func receiveAsync(comps *Components, bundle *bpv7.Bundle) {
	isAdminRecord := bundle.IsAdministrativeRecord()
	if isAdminRecord {
		handleAdministrativeRecord(comps, bundle)
	} else {
		handleBundle(comps, bundle, false)
	}
}

// handleBundle handles a bundle which does not contain an administrative record
func handleBundle(comps *Components, bundle *bpv7.Bundle, new bool) {
	bundleDescriptor, err := comps.Store.InsertBundle(bundle)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": bundle.ID(),
			"error":  err,
		}).Error("Error storing new bundle")
		return
	}

	comps.AAManager.DeliverBundle(bundleDescriptor)

	if new {
		comps.RoutingAlgorithm.NotifyNewBundle(bundleDescriptor, bundle)
	} else {
		comps.RoutingAlgorithm.NotifyReceivedBundle(bundleDescriptor, bundle)
	}

	if dispatch, err := bundleDescriptor.HasConstraint(store.DispatchPending); err == nil && dispatch {
		log.WithField("bundle", bundleDescriptor).Debug("Forwarding received bundle")
		BundleForwarding(comps, bundleDescriptor, bundle)
	}
}

// handleAdministrativeRecord handles reception logic for bundles which contains an administrative record
func handleAdministrativeRecord(comps *Components, bundle *bpv7.Bundle) {
	if comps.RoutingAlgorithm.NotifyReceivedAdministrativeRecord(bundle) && comps.AAManager.DeliverAdministrativeRecord(bundle) {
		bundleDescriptor, err := comps.Store.InsertBundle(bundle)
		if err != nil {
			log.WithFields(log.Fields{
				"bundle": bundle.ID(),
				"error":  err,
			}).Error("Error storing new bundle")
			return
		}

		BundleForwarding(comps, bundleDescriptor, bundle)
	}
}

// ReceiveBundle is to be called when a bundle is received from another node
func ReceiveBundle(comps *Components, bundle *bpv7.Bundle) {
	go receiveAsync(comps, bundle)
}

// NewBundle is to be called if a bundle was created on this node
func NewBundle(comps *Components, bundle *bpv7.Bundle) {
	go handleBundle(comps, bundle, true)
}
