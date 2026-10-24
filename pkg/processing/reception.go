package processing

import (
	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/application_agent"
	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/routing"
	"github.com/dtn7/dtn7-go/pkg/store"
)

func receiveAsync(st *store.BundleStore, bundle *bpv7.Bundle) {
	isAdminRecord := bundle.IsAdministrativeRecord()
	if isAdminRecord {
		handleAdministrativeRecord(st, bundle)
	} else {
		handleBundle(st, bundle, false)
	}
}

// handleBundle handles a bundle which does not contain an administrative record
func handleBundle(st *store.BundleStore, bundle *bpv7.Bundle, new bool) {
	bundleDescriptor, err := st.InsertBundle(bundle)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": bundle.ID(),
			"error":  err,
		}).Error("Error storing new bundle")
		return
	}

	application_agent.GetManagerSingleton().DeliverBundle(bundleDescriptor)

	if new {
		routing.GetAlgorithmSingleton().NotifyNewBundle(bundleDescriptor, bundle)
	} else {
		routing.GetAlgorithmSingleton().NotifyReceivedBundle(bundleDescriptor, bundle)
	}

	if dispatch, err := bundleDescriptor.HasConstraint(store.DispatchPending); err == nil && dispatch {
		log.WithField("bundle", bundleDescriptor).Debug("Forwarding received bundle")
		BundleForwarding(bundleDescriptor, bundle)
	}
}

// handleAdministrativeRecord handles reception logic for bundles which contains an administrative record
func handleAdministrativeRecord(st *store.BundleStore, bundle *bpv7.Bundle) {
	if routing.GetAlgorithmSingleton().NotifyReceivedAdministrativeRecord(bundle) && application_agent.GetManagerSingleton().DeliverAdministrativeRecord(bundle) {
		bundleDescriptor, err := st.InsertBundle(bundle)
		if err != nil {
			log.WithFields(log.Fields{
				"bundle": bundle.ID(),
				"error":  err,
			}).Error("Error storing new bundle")
			return
		}

		BundleForwarding(bundleDescriptor, bundle)
	}
}

// ReceiveBundle is to be called when a bundle is received from another node
func ReceiveBundle(st *store.BundleStore, bundle *bpv7.Bundle) {
	go receiveAsync(st, bundle)
}

// NewBundle is to be called if a bundle was created on this node
func NewBundle(st *store.BundleStore, bundle *bpv7.Bundle) {
	go handleBundle(st, bundle, true)
}
