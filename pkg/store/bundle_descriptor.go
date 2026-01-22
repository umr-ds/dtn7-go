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
	ID                     bpv7.BundleID
	Source                 bpv7.EndpointID
	Destination            bpv7.EndpointID
	ReportTo               bpv7.EndpointID
	IsAdministrativeRecord bool

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

// BundleDescriptor is a lightweight data structure which contains bundle metadata that can be kept in-memory.
// Since a bundle's payload can be arbitrarily large, it does not make sense to keep the entire bundle in memory.
// Instead, all data that is needed for most operations is mirrored to the BundleDescriptor
// and the Bundle itself will only be loaded from disk when absolutely necessary.
// Mirrored metadata can be found in the "Metadata"-field, which s persisted to disk,
// while the other fields contain data that doesn't make sense to persist.
type BundleDescriptor struct {
	// metadata is all data which should be preserved to the database
	metadata   BundleMetadata
	stateMutex sync.RWMutex

	// retentionConstraints as defined by RFC9171 Section 5, see constraints.go for possible values
	retentionConstraints []Constraint
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
		metadata:             metadata,
		retentionConstraints: []Constraint{DispatchPending},
		retain:               true,
		dispatch:             true,
		deleted:              false,
	}
	return &bd
}

// Metadata returns the Metadata associated with this bundle
// If bundle has been deleted, returns BundleDeletedError
func (bd *BundleDescriptor) Metadata() (BundleMetadata, error) {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	if bd.deleted {
		return BundleMetadata{}, NewBundleDeletedError(bd.metadata.ID)
	}

	return bd.metadata, nil
}

// Load loads the entire bundle from disk
// Since bundles can be rather arbitrarily large, this can be very expensive and should only be done when necessary.
// If bundle has been deleted, returns BundleDeletedError
// If there's an error loading the Bundle from disk, returns the causing error.
func (bd *BundleDescriptor) Load() (*bpv7.Bundle, error) {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	if bd.deleted {
		return nil, NewBundleDeletedError(bd.metadata.ID)
	}

	return GetStoreSingleton().loadEntireBundle(bd.metadata.SerialisedFileName)
}

// GetKnownHolders gets the list of EndpointIDs which we know to already have received the bundle.
// If bundle has been deleted, returns BundleDeletedError
func (bd *BundleDescriptor) GetKnownHolders() ([]bpv7.EndpointID, error) {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	if bd.deleted {
		return nil, NewBundleDeletedError(bd.metadata.ID)
	}

	return bd.metadata.KnownHolders, nil
}

// AddKnownHolder adds EndpointIDs to this bundle's list of known recipients.
// If bundle has been deleted, returns BundleDeletedError
func (bd *BundleDescriptor) AddKnownHolder(peers ...bpv7.EndpointID) error {
	bd.stateMutex.Lock()
	defer bd.stateMutex.Unlock()

	if bd.deleted {
		return NewBundleDeletedError(bd.metadata.ID)
	}

	bd.metadata.KnownHolders = append(bd.metadata.KnownHolders, peers...)
	err := GetStoreSingleton().updateBundleMetadata(bd.metadata)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": bd.metadata.IDString,
			"error":  err,
		}).Error("Error syncing bundle metadata")
	} else {
		log.WithFields(log.Fields{
			"bundle": bd.metadata.IDString,
			"peers":  peers,
		}).Debug("Peers added to already sent")
	}

	return nil
}

// AddConstraint adds a Constraint to this bundle and checks if it should be retained/dispatched.
// If the constraint is not one of the listed constants, returns InvalidConstraint
// If bundle has been deleted, returns BundleDeletedError
func (bd *BundleDescriptor) AddConstraint(constraint Constraint) error {
	bd.stateMutex.Lock()
	defer bd.stateMutex.Unlock()

	if bd.deleted {
		return NewBundleDeletedError(bd.metadata.ID)
	}

	if !constraint.Valid() {
		return NewInvalidConstraint(constraint)
	}

	bd.retentionConstraints = append(bd.retentionConstraints, constraint)

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
// If bundle has been deleted, returns BundleDeletedError
func (bd *BundleDescriptor) RemoveConstraint(constraint Constraint) error {
	bd.stateMutex.Lock()
	defer bd.stateMutex.Unlock()

	if bd.deleted {
		return NewBundleDeletedError(bd.metadata.ID)
	}

	bd.dispatch = true
	constraints := make([]Constraint, 0, len(bd.retentionConstraints))
	for _, existingConstraint := range bd.retentionConstraints {
		if existingConstraint != constraint {
			constraints = append(constraints, existingConstraint)

			// if the bundle has the ForwardPending constraint, then it is currently going though the processing pipeline
			// and should not be dispatched again
			if existingConstraint == ForwardPending {
				bd.dispatch = false
			}
		}
	}
	bd.retentionConstraints = constraints
	bd.retain = len(bd.retentionConstraints) > 0

	return nil
}

// ResetConstraints removes all Constraints from this bundle.
// If bundle has been deleted, returns BundleDeletedError
func (bd *BundleDescriptor) ResetConstraints() error {
	bd.stateMutex.Lock()
	defer bd.stateMutex.Unlock()

	if bd.deleted {
		return NewBundleDeletedError(bd.metadata.ID)
	}

	bd.retentionConstraints = make([]Constraint, 0)
	bd.retain = false
	bd.dispatch = true

	return nil
}

// HasConstraint checks if bundle as given Constraint
// If bundle has been deleted, returns BundleDeletedError
func (bd *BundleDescriptor) HasConstraint(constraint Constraint) (bool, error) {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	if bd.deleted {
		return false, NewBundleDeletedError(bd.metadata.ID)
	}

	for _, ownConstraint := range bd.retentionConstraints {
		if ownConstraint == constraint {
			return true, nil
		}
	}

	return false, nil
}

// Dispatch tells, whether this bundle may be considered for dispatching
func (bd *BundleDescriptor) Dispatch() bool {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	return bd.dispatch && !bd.deleted
}

// Retain tells, whether this bundle should be retained (protected from deletion)
func (bd *BundleDescriptor) Retain() bool {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	return bd.retain && !bd.deleted
}

// Deleted tells, whether this bundle has been deleted
// BundleDescriptors may exist for longer than their underlying Bundle
func (bd *BundleDescriptor) Deleted() bool {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	return bd.deleted
}

// Delete deletes this BundleDescriptor's underlying Bundle
// Returns BundleDeletedError if bundle has already been deleted
// Returns HasConstraintsError if bundle has retention constraints
// Returns whatever errors badgerhold returns, when something goes wrong...
func (bd *BundleDescriptor) Delete() error {
	bd.stateMutex.Lock()
	defer bd.stateMutex.Unlock()

	if bd.deleted {
		return NewBundleDeletedError(bd.metadata.ID)
	}

	if len(bd.retentionConstraints) > 0 {
		return NewHasConstraintsError(bd.retentionConstraints)
	}

	bd.deleted = true
	return GetStoreSingleton().deleteBundle(bd.metadata)
}

// Expired tells, whether this bundle's expiry date has been passed
func (bd *BundleDescriptor) Expired() bool {
	return bd.metadata.Expires.Before(time.Now())
}

func (bd *BundleDescriptor) ID() bpv7.BundleID {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	return bd.metadata.ID
}

func (bd *BundleDescriptor) String() string {
	bd.stateMutex.RLock()
	defer bd.stateMutex.RUnlock()

	return bd.metadata.IDString
}
