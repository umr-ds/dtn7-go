package routing

import (
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
	"github.com/dtn7/dtn7-go/pkg/store"
)

const sprayBundleCopiesKey string = "spray_and_wait/copies"

// SprayAndWait routing algorithm (https://doi.org/10.1145/1080139.1080143)
// Implements both basic and binary mode
type SprayAndWait struct {
	// copies is the default value that is attached to bundles that don't have a spray&wait extension block
	copies uint64
	// whether to run in binary-mode
	binary bool
	// stores fore each bundle and peer combination, how many copies should be forwarded
	bundleCopies map[bpv7.BundleID]map[bpv7.EndpointID]uint64

	stateMutex sync.RWMutex
}

func NewSprayAndWait(copies uint64, binary bool) *SprayAndWait {
	router := SprayAndWait{
		copies:       copies,
		binary:       binary,
		bundleCopies: make(map[bpv7.BundleID]map[bpv7.EndpointID]uint64),
	}
	return &router
}

// NotifyPeerAppeared does nothing for this algorithm
func (router *SprayAndWait) NotifyPeerAppeared(_ bpv7.EndpointID) {}

// NotifyPeerDisappeared does nothing for this algorithm
func (router *SprayAndWait) NotifyPeerDisappeared(_ bpv7.EndpointID) {}

func (router *SprayAndWait) NotifyNewBundle(descriptor *store.BundleDescriptor, _ *bpv7.Bundle) {
	log.WithField("bundle", descriptor.ID()).Debug("Spray&Wait handling new bundle")
	setSprayCopies(descriptor, router.copies)
}

func (router *SprayAndWait) NotifyReceivedBundle(descriptor *store.BundleDescriptor, bundle *bpv7.Bundle) {
	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()

	log.WithField("bundle", descriptor.ID()).Debug("Spray&Wait handling received bundle")
	log.WithField("bundle", descriptor.ID()).Debug("Checking for BinarySprayBlock")
	block, err := bundle.ExtensionBlockByType(bpv7.BlockTypeBinarySprayBlock)
	if err != nil {
		log.WithField("bundle", descriptor.ID()).Debug("Bundle has no BinarySprayBlock, using copies 0")
		setSprayCopies(descriptor, 0)
	} else {
		// though the original paper does not specify either way,
		// it seems sensible to honour the value of a BinarySprayBlock, even if we are not running in binary mode
		copies := block.Value.(*bpv7.BinarySprayBlock).Copies()
		log.WithFields(log.Fields{
			"bundle": descriptor.ID(),
			"copies": copies,
		}).Debug("Bundle has BinarySprayBlock, using received copies")
		setSprayCopies(descriptor, copies)
	}
}

// NotifyReceivedAdministrativeRecord does nothing for this algorithm
func (router *SprayAndWait) NotifyReceivedAdministrativeRecord(_ *bpv7.Bundle) bool {
	return true
}

func (router *SprayAndWait) SelectPeersForForwarding(descriptor *store.BundleDescriptor, peers []cla.ConvergenceSender) []cla.ConvergenceSender {
	log.WithField("bundle", descriptor.ID()).Debug("Spray&Wait selecting peers for forwarding")

	// check if the bundle has any copies left before we try anything else
	if !router.hasCopiesLeft(descriptor) {
		return []cla.ConvergenceSender{}
	}

	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()

	// yes, we are getting the same value as before, but we need to make sure someone else didn't change it in the meantime
	// Why do it above? RWMutex.Rlock() can accomodate multiple readers, so if we have a bundle whose copies have reached the cutoff
	// then this will short-circuit and avoid us doing the more expensive write-lock
	copies, ok := getSprayCopies(descriptor)
	if !ok {
		log.WithField("bundle", descriptor.ID()).Debug("Bundle had no saved copies, assuming default")
		copies = router.copies
		setSprayCopies(descriptor, copies)
	}

	if !router.binary && !(copies > 0) {
		log.WithField("bundle", descriptor.ID()).Debug("Value changed since first check. Basic Spray stops at 0 copies remaining")
		return []cla.ConvergenceSender{}
	} else if router.binary && !(copies > 1) {
		log.WithField("bundle", descriptor.ID()).Debug("Value changed since first check. Binary Spray stops at 1 copy remaining")
		return []cla.ConvergenceSender{}
	}

	nPeers := uint64(len(peers))
	if !(nPeers > 0) {
		log.WithField("bundle", descriptor.ID()).Debug("No suitable peers connected")
		return []cla.ConvergenceSender{}
	}

	if router.binary {
		return router.selectBinarySpray(descriptor, copies, peers)
	} else {
		return router.selectBasicSpray(descriptor, copies, peers, nPeers)
	}
}

func (router *SprayAndWait) hasCopiesLeft(descriptor *store.BundleDescriptor) bool {
	router.stateMutex.RLock()
	defer router.stateMutex.RUnlock()

	copies, present := getSprayCopies(descriptor)
	if !present {
		log.WithField("bundle", descriptor.ID()).Warn("Bundle has no saved number of copies. This shouldn't happen")
		return false
	}

	if router.binary {
		log.WithField("bundle", descriptor.ID()).Debug("Binary Spray stops at 1 copy remaining")
		return copies > 1
	} else {
		log.WithField("bundle", descriptor.ID()).Debug("Basic Spray stops at 0 copies remaining")
		return copies > 0
	}
}

// selectBasicSpray runs algorithm in basic mode
// The originating node will spray the configured number od copies to other nodes, but other nodes don't replicate the bundle themselves
// A second forwarding hop only happen through direct transmission (when a carrying node encounters the recipient)
func (router *SprayAndWait) selectBasicSpray(descriptor *store.BundleDescriptor, copies uint64, peers []cla.ConvergenceSender, nPeers uint64) []cla.ConvergenceSender {
	log.WithField("bundle", descriptor.ID()).Debug("Spray&Wait running in basic mode")
	var remainingCopies uint64
	var selectedPeers []cla.ConvergenceSender
	if nPeers <= copies {
		log.WithField("bundle", descriptor.ID()).Debug("Fewer peers than remaining copies, sending to everyone")
		remainingCopies = copies - nPeers
		selectedPeers = peers
	} else {
		log.WithField("bundle", descriptor.ID()).Debug("More peers than remaining copies")
		remainingCopies = 0
		selectedPeers = peers[0:copies]
	}

	setSprayCopies(descriptor, remainingCopies)
	log.WithFields(log.Fields{
		"bundle":           descriptor.ID(),
		"remaining copies": remainingCopies,
		"selected peers":   selectedPeers,
	}).Debug("Basic Spray selected peers for forwarding")
	return selectedPeers
}

// selectBinarySpray runs the algorithm in binary mode
// The originating node starts with l copies, and every time it forwards the bundle, it is tagged with n/2 copies, while the transmitting node keeps the other n/2 for itself
// Since we need to modify the bundle and attach an appropriate extension block, we can only choose one peer per routing invocation.
func (router *SprayAndWait) selectBinarySpray(descriptor *store.BundleDescriptor, copies uint64, peers []cla.ConvergenceSender) []cla.ConvergenceSender {
	log.WithField("bundle", descriptor.ID()).Debug("Spray&Wait running in binary mode")

	bundleCopies, present := router.bundleCopies[descriptor.ID()]
	if !present {
		bundleCopies = make(map[bpv7.EndpointID]uint64)
		router.bundleCopies[descriptor.ID()] = bundleCopies
	}

	selected := make([]cla.ConvergenceSender, 0, len(peers))
	for _, peer := range peers {
		log.WithFields(log.Fields{
			"bundle": descriptor.ID(),
			"peer":   peer.GetPeerEndpointID(),
		}).Debug("Binary Spray checks peer")
		if copies <= 1 {
			log.WithFields(log.Fields{
				"bundle": descriptor.ID(),
				"peer":   peer.GetPeerEndpointID(),
			}).Debug("No more copies left, aborting")
			break
		}

		sendCopies := copies / 2
		retainedCopies := copies / 2
		// if the number of copies is odd, we retain one more than we give away
		if (copies % 2) != 0 {
			retainedCopies += 1
		}
		copies = retainedCopies

		log.WithFields(log.Fields{
			"bundle": descriptor.ID(),
			"peer":   peer.GetPeerEndpointID(),
			"copies": sendCopies,
		}).Debug("Peer will receive copies")
		bundleCopies[peer.GetPeerEndpointID()] = sendCopies
		selected = append(selected, peer)
	}

	setSprayCopies(descriptor, copies)

	log.WithFields(log.Fields{
		"bundle": descriptor.ID(),
		"peers":  selected,
	}).Debug("Binary spray selected peers for forwarding")
	return selected
}

func (router *SprayAndWait) ModifyHeaders(descriptor *store.BundleDescriptor, headers *bpv7.PartialBundle, peer cla.ConvergenceSender) error {
	if !router.binary {
		// basic spray does not need to modify bundle
		return nil
	}

	router.stateMutex.Lock()
	defer router.stateMutex.Unlock()

	log.WithFields(log.Fields{
		"bundle": descriptor.ID(),
		"peer":   peer.GetPeerEndpointID(),
	}).Debug("Binary Spray modifying bundle headers")

	bundleCopies, present := router.bundleCopies[descriptor.ID()]
	if !present {
		return NewNoCopiesError(descriptor.ID(), peer.GetPeerEndpointID())
	}

	peerCopies, present := bundleCopies[peer.GetPeerEndpointID()]
	if !present {
		return NewNoCopiesError(descriptor.ID(), peer.GetPeerEndpointID())
	}

	delete(bundleCopies, peer.GetPeerEndpointID())

	// remove exiting BinarySprayBlock(s)
	headers.RemoveExtensionBlocks(bpv7.BlockTypeBinarySprayBlock)

	// add a new one
	block := bpv7.NewCanonicalBlock(0, 0, bpv7.NewBinarySprayBlock(peerCopies))
	err := headers.AddExtensionBlock(block)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": descriptor.ID(),
			"peer":   peer.GetPeerEndpointID(),
			"error":  err,
		}).Error("Error adding block to headers")
		return err
	}

	return nil
}

func setSprayCopies(descriptor *store.BundleDescriptor, copies uint64) {
	err := descriptor.SetMiscData(sprayBundleCopiesKey, copies)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": descriptor.ID(),
			"error":  err,
		}).Error("Spray&Wait could not set bundle copies")
	}
}

func getSprayCopies(descriptor *store.BundleDescriptor) (uint64, bool) {
	data, ok := descriptor.GetMiscData(sprayBundleCopiesKey)
	if !ok {
		return 0, false
	}
	copies := data.(uint64)
	return copies, true
}

type NoCopiesError struct {
	bundle bpv7.BundleID
	peer   bpv7.EndpointID
}

func NewNoCopiesError(bundle bpv7.BundleID, peer bpv7.EndpointID) error {
	return NoCopiesError{bundle, peer}
}

func (e NoCopiesError) Error() string {
	return fmt.Sprintf("no copies stored for combination of bundle %v and peer %v", e.bundle, e.peer)
}
