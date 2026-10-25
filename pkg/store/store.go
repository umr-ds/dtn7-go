// SPDX-FileCopyrightText: 2023, 2024 Artur Sterz
// SPDX-FileCopyrightText: 2023, 2024, 2025, 2026 Markus Sommer
// SPDX-FileCopyrightText: 2026 Philipp Jahn
//
// SPDX-License-Identifier: GPL-3.0-or-later

// Package store implements on-disk persistence for bundles and their metadata
// Uses Badgerhold (github.com/timshannon/badgerhold) for persisting metadata.
// Bundles are stored in CBOR-serialized form on-disk.
package store

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/timshannon/badgerhold/v4"

	"github.com/dtn7/cboring"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
)

type BundleStore struct {
	nodeID          bpv7.EndpointID
	metadataStore   *badgerhold.Store
	bundleDirectory string
	bundles         map[bpv7.BundleID]*BundleDescriptor
	stateMutex      sync.RWMutex
}

func NewStore(nodeID bpv7.EndpointID, path string) (*BundleStore, error) {
	opts := badgerhold.DefaultOptions
	opts.Dir = path
	opts.ValueDir = path
	opts.Logger = log.StandardLogger()

	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}

	badgerStore, err := badgerhold.Open(opts)
	if err != nil {
		return nil, err
	}

	bundleDirectory := filepath.Join(path, "bundles")
	if err := os.MkdirAll(bundleDirectory, 0700); err != nil {
		return nil, err
	}

	store := BundleStore{
		nodeID:          nodeID,
		metadataStore:   badgerStore,
		bundleDirectory: bundleDirectory,
	}

	allBundles, err := store.loadAll()
	log.WithField("bundles", len(allBundles)).Debug("Got all bundles from disk")
	if err != nil {
		return nil, err
	}
	store.bundles = make(map[bpv7.BundleID]*BundleDescriptor, len(allBundles))
	for _, bundle := range allBundles {
		store.bundles[bundle.ID()] = bundle
	}

	return &store, nil
}

func closeWithError(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.WithError(err).Error("Error closing resource")
	}
}

func (bst *BundleStore) Shutdown() error {
	log.Info("Shutting down store")
	log.WithField("bundles", len(bst.bundles)).Debug("Bundles in store at shutdown")
	err := bst.metadataStore.Close()
	log.Info("Store shut down")
	return err
}

func (bst *BundleStore) loadAll() ([]*BundleDescriptor, error) {
	bundles := make([]BundleMetadata, 0)
	err := bst.metadataStore.Find(&bundles, &badgerhold.Query{})
	if err != nil {
		return nil, err
	}
	descriptors := make([]*BundleDescriptor, 0, len(bundles))
	for _, metadata := range bundles {
		descriptors = append(descriptors, NewBundleDescriptor(metadata, bst))
	}
	return descriptors, nil
}

// GetBundleDescriptor return BundleDescriptor for the given BundleID.
// If no bundle with the given ID is in the store, method will return NoSuchBundleError
func (bst *BundleStore) GetBundleDescriptor(bundleId bpv7.BundleID) (*BundleDescriptor, error) {
	log.WithField("bid", bundleId).Debug("Getting BundleDescriptor from store")
	bst.stateMutex.RLock()
	defer bst.stateMutex.RUnlock()
	descriptor, ok := bst.bundles[bundleId]
	if !ok {
		log.WithField("bid", bundleId).Debug("Bundle not found")
		return nil, NewNoSuchBundleError(bundleId)
	}

	if descriptor.Deleted() {
		log.WithField("bid", bundleId).Debug("Bundle has been deleted")
		return nil, NewNoSuchBundleError(bundleId)
	}

	return descriptor, nil
}

// GetWithConstraint loads all BundleDescriptors which have the given Constraint set.
// For an explanation of retention constraints, see RFC9171 Section 5.
func (bst *BundleStore) GetWithConstraint(constraint Constraint) []*BundleDescriptor {
	log.WithField("constraint", constraint).Debug("Getting BundleDescriptors with constraint")
	bst.stateMutex.RLock()
	defer bst.stateMutex.RUnlock()

	bundles := make([]*BundleDescriptor, 0)
	for _, bundle := range bst.bundles {
		hasConstraint, err := bundle.HasConstraint(constraint)
		if err != nil {
			continue
		}
		if hasConstraint {
			bundles = append(bundles, bundle)
		}
	}

	return bundles
}

// GetDispatchable loads all BundleDescriptors which refer to currently dispatchable bundles.
// That is bundles, which are not already in the process of being forwarded.
func (bst *BundleStore) GetDispatchable() []*BundleDescriptor {
	log.Debug("Getting dispatchable bundles")
	bst.stateMutex.RLock()
	defer bst.stateMutex.RUnlock()

	bundles := make([]*BundleDescriptor, 0)
	for _, bundle := range bst.bundles {
		if bundle.Dispatch() {
			bundles = append(bundles, bundle)
		}
	}

	return bundles
}

func (bst *BundleStore) loadEntireBundle(filename string) (*bpv7.Bundle, error) {
	path := filepath.Join(bst.bundleDirectory, filename)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer closeWithError(f)

	bundle, err := bpv7.ParseBundle(f)

	return bundle, nil
}

func (bst *BundleStore) loadPartialBundle(filename string, wantedBlocks []bpv7.BlockType) (*bpv7.PartialBundle, error) {
	path := filepath.Join(bst.bundleDirectory, filename)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer closeWithError(f)

	extensionBlocks, err := bpv7.ParsePartialBundle(f, wantedBlocks)

	return extensionBlocks, err
}

// insertNewBundleUnsafe stores a new bundle on disk and creates a new BundleDescriptor
// This method is NOT threadsafe - you must have locked the stateMutex BEFORE calling this.
func (bst *BundleStore) insertNewBundleUnsafe(bundle *bpv7.Bundle) (*BundleDescriptor, error) {
	log.WithField("bundle", bundle.ID().String()).Debug("Inserting new bundle")
	lifetimeDuration := time.Millisecond * time.Duration(bundle.PrimaryBlock.Lifetime)
	serialisedFileName := fmt.Sprintf("%x", sha256.Sum256([]byte(bundle.ID().String())))
	metadata := BundleMetadata{
		ID:                     bundle.ID(),
		IDString:               bundle.ID().String(),
		Source:                 bundle.PrimaryBlock.SourceNode,
		Destination:            bundle.PrimaryBlock.Destination,
		ReportTo:               bundle.PrimaryBlock.ReportTo,
		IsAdministrativeRecord: bundle.IsAdministrativeRecord(),
		MiscellaneousData:      make(map[string]any),
		KnownHolders:           []bpv7.EndpointID{bst.nodeID},
		Expires:                bundle.PrimaryBlock.CreationTimestamp.DtnTime().Time().Add(lifetimeDuration),
		SerialisedFileName:     serialisedFileName,
	}

	if metadata.IsAdministrativeRecord {
		record, err := bundle.AdministrativeRecord()
		if err != nil {
			log.WithFields(log.Fields{
				"bundle": metadata.ID.String(),
				"error":  err,
			}).Error("Unable to read administrative record")
		} else {
			metadata.AdministrativeRecordType = record.RecordType()
		}
	}

	if previousNodeBlock, err := bundle.ExtensionBlockByType(bpv7.BlockTypePreviousNodeBlock); err == nil {
		previousNode := previousNodeBlock.Value.(*bpv7.PreviousNodeBlock).Endpoint()
		metadata.KnownHolders = append(metadata.KnownHolders, previousNode)
		log.WithFields(log.Fields{
			"bundle": metadata.ID,
			"sender": previousNode,
		}).Debug("Added sender to known holders")
	}

	err := bst.metadataStore.Insert(metadata.IDString, metadata)
	if err != nil {
		return nil, err
	}

	serialisedPath := filepath.Join(bst.bundleDirectory, serialisedFileName)
	f, err := os.Create(serialisedPath)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": metadata.IDString,
			"error":  err,
		}).Error("Error opening file to store serialised bundle. Deleting...")
		delErr := bst.deleteFromDiskUnsafe(metadata)
		if delErr != nil {
			log.WithFields(log.Fields{
				"bundle": metadata.IDString,
				"error":  delErr,
			}).Error("Error deleting BundleDescriptor. Something is very wrong")
			err = multierror.Append(err, delErr)
		}
		return nil, err
	}
	defer closeWithError(f)

	w := bufio.NewWriter(f)
	err = cboring.Marshal(bundle, w)
	if err != nil {
		return nil, err
	}
	err = w.Flush()
	if err != nil {
		return nil, err
	}

	descriptor := NewBundleDescriptor(metadata, bst)
	bst.bundles[bundle.ID()] = descriptor

	return descriptor, nil
}

// InsertBundle inserts a bundle into the store.
//
// If the bundle is new, then a new BundleDescriptor wil be generated and store in the database.
// The bundle itself will be stored in CBOR serialized form.
//
// If the bundle is already in the store, then we extract the bpv7.PreviousNodeBlock to see whe we received it from
// and add them to the list of node that we know to already have this bundle.
func (bst *BundleStore) InsertBundle(bundle *bpv7.Bundle) (*BundleDescriptor, error) {
	log.WithField("bundle", bundle.ID()).Debug("Inserting bundle")
	bst.stateMutex.Lock()
	defer bst.stateMutex.Unlock()

	descriptor, ok := bst.bundles[bundle.ID()]
	if !ok {
		log.WithField("bundle", bundle.ID()).Debug("Bundle not in store")
		return bst.insertNewBundleUnsafe(bundle)
	}

	log.WithField("bundle", bundle.ID()).Debug("Bundle already exists, updating metadata")

	if previousNodeBlock, err := bundle.ExtensionBlockByType(bpv7.BlockTypePreviousNodeBlock); err == nil {
		previousNode := previousNodeBlock.Value.(*bpv7.PreviousNodeBlock).Endpoint()
		err = descriptor.AddKnownHolder(previousNode)
		if err != nil {
			return nil, err
		}
	}

	return descriptor, nil
}

func (bst *BundleStore) updateBundleMetadata(bundleMetadata BundleMetadata) error {
	return bst.metadataStore.Update(bundleMetadata.IDString, bundleMetadata)
}

// deleteFromDiskUnsafe deletes bundle data & metadata from disk
// This method is NOT threadsafe - you must have locked the stateMutex BEFORE calling this.
func (bst *BundleStore) deleteFromDiskUnsafe(metadata BundleMetadata) error {
	var multiErr *multierror.Error
	multiErr = multierror.Append(multiErr, bst.metadataStore.Delete(metadata.IDString, metadata))
	serialisedPath := filepath.Join(bst.bundleDirectory, metadata.SerialisedFileName)
	multiErr = multierror.Append(multiErr, os.Remove(serialisedPath))
	return multiErr.ErrorOrNil()
}

// deleteBundle deletes bundle from store map, metadata database and the serialized bundle from disk.
func (bst *BundleStore) deleteBundle(metadata BundleMetadata) {
	log.WithField("bundle", metadata.ID).Debug("Deleting bundle")

	bst.stateMutex.Lock()
	defer bst.stateMutex.Unlock()

	delete(bst.bundles, metadata.ID)

	err := bst.deleteFromDiskUnsafe(metadata)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": metadata.ID.String(),
			"error":  err,
		}).Error("Error deleting bundle")
	}
}

// GarbageCollect deletes all bundles which are expired (their creation timestamp + lifetime is in the past)
// and are not currently marked for retention.
func (bst *BundleStore) GarbageCollect() {
	log.Debug("Garbage collecting store")
	bst.stateMutex.Lock()
	defer bst.stateMutex.Unlock()

	for _, bundle := range bst.bundles {
		if bundle.Expired() && !bundle.Retain() {
			err := bundle.Delete(false)
			if err != nil {
				log.WithField("error", err).Error("Error garbage collecting bundle")
			}
		}
	}
}
