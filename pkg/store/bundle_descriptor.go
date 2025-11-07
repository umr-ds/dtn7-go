package store

import (
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
	AlreadySentTo []bpv7.EndpointID

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
	Metadata BundleMetadata

	// RetentionConstraints as defined by RFC9171 Section 5, see constraints.go for possible values
	RetentionConstraints []Constraint
	// should this bundle be retained, i.e. protected from deletion
	// bundle's with constraints are also currently being processed
	Retain bool
	// should this bundle be dispatched?
	Dispatch bool
}

func NewBundleDescriptor(metadata BundleMetadata) *BundleDescriptor {
	bd := BundleDescriptor{
		Metadata:             metadata,
		RetentionConstraints: []Constraint{DispatchPending},
		Retain:               true,
		Dispatch:             true,
	}
	return &bd
}

// Load loads the entire bundle from disk
// Since bundles can be rather arbitrarily large, this can be very expensive and should only be done when necessary.
func (bd *BundleDescriptor) Load() (*bpv7.Bundle, error) {
	return GetStoreSingleton().loadEntireBundle(bd.Metadata.SerialisedFileName)
}

// GetAlreadySent gets the list of EndpointIDs which we know to already have received the bundle.
func (bd *BundleDescriptor) GetAlreadySent() []bpv7.EndpointID {
	// TODO: refresh current state from db
	// TODO: give better name, since we might also know from receiving the bundle from someone else
	return bd.Metadata.AlreadySentTo
}

// AddAlreadySent adds EndpointIDs to this bundle's list of known recipients.
func (bd *BundleDescriptor) AddAlreadySent(peers ...bpv7.EndpointID) {
	bd.Metadata.AlreadySentTo = append(bd.Metadata.AlreadySentTo, peers...)
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
// Changes are synced to disk.
func (bd *BundleDescriptor) AddConstraint(constraint Constraint) error {
	// check if value is valid constraint
	if constraint < DispatchPending || constraint > ReassemblyPending {
		return NewInvalidConstraint(constraint)
	}

	bd.RetentionConstraints = append(bd.RetentionConstraints, constraint)
	bd.Retain = true
	bd.Dispatch = constraint != ForwardPending
	return GetStoreSingleton().updateBundleMetadata(bd.Metadata)
}

// RemoveConstraint removes a Constraint from this bundle and checks if it should be retained/dispatched.
// Changes are synced to disk.
func (bd *BundleDescriptor) RemoveConstraint(constraint Constraint) error {
	constraints := make([]Constraint, 0, len(bd.RetentionConstraints))
	for _, existingConstraint := range bd.RetentionConstraints {
		if existingConstraint != constraint {
			constraints = append(constraints, existingConstraint)
		}
	}
	bd.RetentionConstraints = constraints
	bd.Retain = len(bd.RetentionConstraints) > 0
	bd.Dispatch = constraint == ForwardPending
	return GetStoreSingleton().updateBundleMetadata(bd.Metadata)
}

// ResetConstraints removes all Constraints from this bundle.
// Changes are synced to disk.
func (bd *BundleDescriptor) ResetConstraints() error {
	bd.RetentionConstraints = make([]Constraint, 0)
	bd.Retain = false
	bd.Dispatch = true
	return GetStoreSingleton().updateBundleMetadata(bd.Metadata)
}

func (bd *BundleDescriptor) String() string {
	return bd.Metadata.IDString
}
