// SPDX-FileCopyrightText: 2019, 2020, 2022 Alvar Penning
// SPDX-FileCopyrightText: 2019, 2021, 2025, 2026 Markus Sommer
//
// SPDX-License-Identifier: GPL-3.0-or-later

package bpv7

import (
	"io"

	"github.com/dtn7/cboring"
)

// BinarySprayBlock contains metadata used by the "Binary Spray & Wait" routing algorithm.
//
// It is attached to a bundle to let the receiving peers know of the bundle's remaining copies,
// that is the number of times this bundle may be forwarded to non-recipient nodes.
// Each node in the forwarding chain is expected to update this on a successful forward and halve the remaining copies.
//
// NOTE:
// This is a custom extension block, and not part of the original bpv7 specification.
// It is currently assigned the block type code 192,
// which the specification sets aside for "private and/or experimental use"
type BinarySprayBlock uint64

func NewBinarySprayBlock(copies uint64) *BinarySprayBlock {
	newBlock := BinarySprayBlock(copies)
	return &newBlock
}

func (bsb *BinarySprayBlock) BlockTypeCode() BlockType {
	return BlockTypeBinarySprayBlock
}

func (bsb *BinarySprayBlock) BlockTypeName() string {
	return "Binary Spray Routing Block"
}

func (bsb *BinarySprayBlock) CheckValid() error {
	return nil
}

func (bsb *BinarySprayBlock) CheckContextValid(*Bundle) error {
	return nil
}

func (bsb *BinarySprayBlock) MarshalCbor(w io.Writer) error {
	return cboring.WriteUInt(uint64(*bsb), w)
}

func (bsb *BinarySprayBlock) UnmarshalCbor(r io.Reader) error {
	if us, err := cboring.ReadUInt(r); err != nil {
		return err
	} else {
		*bsb = BinarySprayBlock(us)
		return nil
	}
}

func (bsb *BinarySprayBlock) Copies() uint64 {
	return uint64(*bsb)
}
