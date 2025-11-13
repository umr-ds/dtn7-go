package store

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
)

// BundleMetadata is metadata mirrored from a Bundle's primary-/extensionblocks that is used throughout the program.
// We don't want to keep the entire bundle in memory, so we only keep this selected subset of data.
type BundleMetadata struct {
	ID          bpv7.BundleID
	Source      bpv7.EndpointID
	Destination bpv7.EndpointID
	ReportTo    bpv7.EndpointID

	// Bundle's ID in string-form. Used as the database primary-key. Return-value of ID.String()
	IDString string `badgerhold:"key"`

	// Node IDs of peers which already have this bundle
	// By tracking these, we can avoid wasting bandwidth by sending bundles to nodes which already have them.
	KnownHolders []bpv7.EndpointID

	// TTL after which the bundle will be deleted - assuming Retain == false
	Expires time.Time
	// filename of the serialised bundle on-disk
	SerialisedFileName string
}

// BundleDescriptor is a "lightweight" data structure which contains bundle metadata that can be kept in-memory.
// Since a bundle's payload can be arbitrarily large, it does not make sense to keep the entire bundle in memory.
// Instead, all data that is needed for most operations is mirrored to the BundleDescriptor
// and the Bundle itself will only be loaded from disk when absolutely necessary.
// Mirrored metadata can be found in the "Metadata"-field, which s persisted to disk,
// while the other fields contain data that doesn't make sense to persist.
type BundleDescriptor struct {
	// Metadata is all data which should be preserved to the database
	Metadata   BundleMetadata
	stateMutex sync.RWMutex

	// RetentionConstraints as defined by RFC9171 Section 5, see constraints.go for possible values
	RetentionConstraints []Constraint
	// should this bundle be retained, i.e. protected from deletion
	// bundle's with constraints are also currently being processed
	retain bool
	// should this bundle be dispatched?
	dispatch bool
	// has this bundle been deleted
	deleted bool
}

func NewBundleDescriptor(metadata BundleMetadata) *BundleDescriptor {
	bd := BundleDescriptor{
		Metadata:             metadata,
		RetentionConstraints: []Constraint{DispatchPending},
		retain:               true,
		dispatch:             true,
		deleted:              false,
	}
	return &bd
}

// Load loads the entire bundle from disk
// Since bundles can be rather arbitrarily large, this can be very expensive and should only be done when necessary.
func (bd *BundleDescriptor) Load() (*bpv7.Bundle, error) {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	return GetStoreSingleton().loadEntireBundle(bd.Metadata.SerialisedFileName)
}

// GetKnownHolders gets the list of EndpointIDs which we know to already have received the bundle.
func (bd *BundleDescriptor) GetKnownHolders() []bpv7.EndpointID {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	return bd.Metadata.KnownHolders
}

// AddKnownHolder adds EndpointIDs to this bundle's list of known recipients.
func (bd *BundleDescriptor) AddKnownHolder(peers ...bpv7.EndpointID) {
	bd.stateMutex.Lock()
	defer bd.stateMutex.Unlock()

	bd.Metadata.KnownHolders = append(bd.Metadata.KnownHolders, peers...)
	err := GetStoreSingleton().updateBundleMetadata(bd.Metadata)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": bd.Metadata.IDString,
			"error":  err,
		}).Error("Error syncing bundle metadata")
	} else {
		log.WithFields(log.Fields{
			"bundle": bd.Metadata.IDString,
			"peers":  peers,
		}).Debug("Peers added to already sent")
	}
}

// AddConstraint adds a Constraint to this bundle and checks if it should be retained/dispatched.
func (bd *BundleDescriptor) AddConstraint(constraint Constraint) error {
	bd.stateMutex.Lock()
	defer bd.stateMutex.Unlock()

	if !constraint.Valid() {
		return NewInvalidConstraint(constraint)
	}

	bd.RetentionConstraints = append(bd.RetentionConstraints, constraint)

	// if there's at least one constraint, the bundle should be retained
	bd.retain = true

	// if the bundle has the ForwardPending constraint, then it is currently going though the processing pipeline
	// and should not be dispatched again
	if constraint == ForwardPending {
		bd.dispatch = false
	}
	return nil
}

// RemoveConstraint removes a Constraint from this bundle and checks if it should be retained/dispatched.
func (bd *BundleDescriptor) RemoveConstraint(constraint Constraint) {
	bd.stateMutex.Lock()
	defer bd.stateMutex.Unlock()

	bd.dispatch = true
	constraints := make([]Constraint, 0, len(bd.RetentionConstraints))
	for _, existingConstraint := range bd.RetentionConstraints {
		if existingConstraint != constraint {
			constraints = append(constraints, existingConstraint)

			// if the bundle has the ForwardPending constraint, then it is currently going though the processing pipeline
			// and should not be dispatched again
			if existingConstraint == ForwardPending {
				bd.dispatch = false
			}
		}
	}
	bd.RetentionConstraints = constraints
	bd.retain = len(bd.RetentionConstraints) > 0
}

// ResetConstraints removes all Constraints from this bundle.
func (bd *BundleDescriptor) ResetConstraints() {
	bd.stateMutex.Lock()
	defer bd.stateMutex.Unlock()

	bd.RetentionConstraints = make([]Constraint, 0)
	bd.retain = false
	bd.dispatch = true
}

func (bd *BundleDescriptor) Dispatch() bool {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	return bd.dispatch
}

func (bd *BundleDescriptor) Retain() bool {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	return bd.retain
}

func (bd *BundleDescriptor) Deleted() bool {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	return bd.deleted
}

func (bd *BundleDescriptor) Delete() error {
	bd.stateMutex.Lock()
	defer bd.stateMutex.Unlock()

	bd.deleted = true
	return GetStoreSingleton().DeleteBundle(bd.Metadata)
}

func (bd *BundleDescriptor) String() string {
	return bd.Metadata.IDString
}
