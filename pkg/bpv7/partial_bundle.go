// SPDX-FileCopyrightText: 2026 Markus Sommer
// SPDX-FileCopyrightText: 2026 Philipp Jahn
//
// SPDX-License-Identifier: GPL-3.0-or-later

package bpv7

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"

	"github.com/dtn7/cboring"
)

// PartialBundle is a partially loaded bundle
type PartialBundle struct {
	PrimaryBlock    PrimaryBlock
	ExtensionBlocks []CanonicalBlock
}

// BundleHeaders creates a PartialBundle from a Bundle by copying the PrimaryBlock and ExtensionBlocks
// IMPORTANT: depending on the underlying implementation, any CanonicalBlock might hold a pointer to an ExtensionBlock.
// So you should NEVER modify existing blocks.
// Instead, use RemoveExtensionBlocks and AddExtensionBlock to remove and (re)add blocks
func BundleHeaders(bundle *Bundle) *PartialBundle {
	pb := PartialBundle{
		PrimaryBlock:    bundle.PrimaryBlock,
		ExtensionBlocks: make([]CanonicalBlock, len(bundle.ExtensionBlocks)),
	}
	for i, extensionBlock := range bundle.ExtensionBlocks {
		pb.ExtensionBlocks[i] = extensionBlock
	}
	return &pb
}

// BundleFromPartialBundle reconstructs a "full" bundle from a PartialBundle and a payload
func BundleFromPartialBundle(partial *PartialBundle, payload CanonicalBlock) *Bundle {
	bundle := Bundle{
		PrimaryBlock:    partial.PrimaryBlock,
		ExtensionBlocks: partial.ExtensionBlocks,
		PayloadBlock:    payload,
	}
	return &bundle
}

// FindExtensionBlock retrieves an extensionBlock from the PartialBundle based on its type
func FindExtensionBlock[T ExtensionBlock](partialBundle *PartialBundle, eb *T) error {
	for _, extensionBlock := range partialBundle.ExtensionBlocks {
		if extensionBlock.TypeCode() == (*eb).BlockTypeCode() {
			var ok bool
			*eb, ok = extensionBlock.Value.(T)
			if ok {
				return nil
			}
		}
	}
	return errors.New("could not find block")
}

// ExtensionBlocksByType returns all this Bundle's canonical extension blocks which match the requested block type code.
// If no such block was found, an error will be returned.
func (b *PartialBundle) ExtensionBlocksByType(blockType BlockType) (cbs []*CanonicalBlock, err error) {
	for i := 0; i < len(b.ExtensionBlocks); i++ {
		cb := &b.ExtensionBlocks[i]
		if cb.TypeCode() == blockType {
			cbs = append(cbs, cb)
		}
	}

	if len(cbs) == 0 {
		cbs = nil
		err = fmt.Errorf("no CanonicalBlock with block type %d was found in Bundle", blockType)
	}
	return
}

// ExtensionBlockByType returns the first Canonical Block which matches the requested type code.
// If there is no such Block an error will be returned.
func (b *PartialBundle) ExtensionBlockByType(blockType BlockType) (*CanonicalBlock, error) {
	for _, extensionBlock := range b.ExtensionBlocks {
		if extensionBlock.TypeCode() == blockType {
			return &extensionBlock, nil
		}
	}
	return nil, fmt.Errorf("no CanonicalBlock with block type %d was found in PartialBundle", blockType)
}

// HasExtensionBlock checks if a ExtensionBlock for some block type number is present.
func (b *PartialBundle) HasExtensionBlock(blockType BlockType) bool {
	_, err := b.ExtensionBlocksByType(blockType)
	return err == nil
}

// sortExtensionBlocks sorts the extension blocks.
// This method is called internally after block modification, e.g., in MustNewBundle or Bundle.AddExtensionBlock.
func (b *PartialBundle) sortExtensionBlocks() {
	sort.Sort(canonicalBlockNumberSort(b.ExtensionBlocks))
}

// AddExtensionBlock adds a new ExtensionBlock.
// The block number will be calculated and overwritten within this method.
func (b *PartialBundle) AddExtensionBlock(block CanonicalBlock) error {
	// TODO: return error if we try to add a block which already exists
	var blockNumbers []uint64
	for i := 0; i < len(b.ExtensionBlocks); i++ {
		blockNumbers = append(blockNumbers, b.ExtensionBlocks[i].BlockNumber)
	}

	var blockNumber uint64 = 1
	if block.Value.BlockTypeCode() != BlockTypePayloadBlock {
		blockNumber = 2
	}

	for {
		flag := !slices.Contains(blockNumbers, blockNumber)

		if flag {
			break
		} else {
			blockNumber += 1
		}
	}

	block.BlockNumber = blockNumber

	b.ExtensionBlocks = append(b.ExtensionBlocks, block)
	b.sortExtensionBlocks()
	return nil
}

// RemoveExtensionBlocks removes all ExtensionBlocks with the given blockType.
func (b *PartialBundle) RemoveExtensionBlocks(blockType BlockType) {
	retain := make([]CanonicalBlock, 0, len(b.ExtensionBlocks))
	for _, extensionBlock := range b.ExtensionBlocks {
		if extensionBlock.Value.BlockTypeCode() != blockType {
			retain = append(retain, extensionBlock)
		}
	}
	b.ExtensionBlocks = retain
}

// ParsePartialBundle  reads the specified parts of a CBOR encoded Bundle from a Reader ito a PartialBundle.
func ParsePartialBundle(r io.Reader, wantedExtensionBlocks []BlockType) (*PartialBundle, error) {
	var primary PrimaryBlock
	var extensionBlocks []CanonicalBlock
	if err := cboring.ReadExpect(cboring.IndefiniteArray, r); err != nil {
		return nil, err
	}

	if err := cboring.Unmarshal(&primary, r); err != nil {
		return nil, fmt.Errorf("PrimaryBlock failed: %v", err)
	}
	for {
		cb := CanonicalBlock{}

		if wanted, err := cb.UnmarshalWantedBlock(r, wantedExtensionBlocks); err == cboring.FlagBreakCode {
			break
		} else if err != nil {
			return nil, fmt.Errorf("CanonicalBlock failed: %v", err)
		} else if cb.TypeCode() == BlockTypePayloadBlock {
			break
		} else if wanted {
			if index := slices.Index(wantedExtensionBlocks, cb.TypeCode()); index >= 0 {
				wantedExtensionBlocks = slices.Delete(wantedExtensionBlocks, index, index+1)
				extensionBlocks = append(extensionBlocks, cb)
				if len(wantedExtensionBlocks) == 0 {
					break
				}
			}
		}
	}
	if err := primary.CheckValid(); err != nil {
		return nil, err
	}
	for i := range extensionBlocks {
		if err := extensionBlocks[i].CheckValid(); err != nil {
			return nil, err
		}
	}
	return &PartialBundle{PrimaryBlock: primary, ExtensionBlocks: extensionBlocks}, nil
}
