// SPDX-FileCopyrightText: 2018, 2019, 2020 Alvar Penning
//
// SPDX-License-Identifier: GPL-3.0-or-later

package bpv7

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/dtn7/cboring"
	"github.com/hashicorp/go-multierror"
)

type BlockType uint64

// Sorted list of all known block type codes to prevent double usage.
const (
	// BlockTypePayloadBlock is the block type code for a Payload Block, bpv7/payload_block.go
	BlockTypePayloadBlock BlockType = 1

	// BlockTypeBlockIntegrityBlock is the block type code for an Integrity Block
	BlockTypeBlockIntegrityBlock BlockType = 3

	// BlockTypeBlockConfidentialityBlock is the block type code for a Confidentiality Block
	BlockTypeBlockConfidentialityBlock BlockType = 4

	// BlockTypePreviousNodeBlock is the block type code for a Previous Node Block, bpv7/extension_block_previous_node.go
	BlockTypePreviousNodeBlock BlockType = 6

	// BlockTypeBundleAgeBlock is the block type code for a Bundle Age Block, bpv7/extension_block_bundle_age.go
	BlockTypeBundleAgeBlock BlockType = 7

	// BlockTypeHopCountBlock is the block type code for a Hop Count Block, bpv7/extension_block_hop_count.go
	BlockTypeHopCountBlock BlockType = 10

	// BlockTypeBinarySprayBlock is the custom block type code for a BinarySprayBlock, bpv7/extension_block_spray.go
	BlockTypeBinarySprayBlock BlockType = 192

	// BlockTypeDTLSRBlock is the custom block type code for a DTLSRBlock, bpv7/extension_block_dtlsr.go
	BlockTypeDTLSRBlock BlockType = 193

	// BlockTypeProphetBlock is the custom block type code for a ProphetBlock, bpv7/extension_block_prophet.go
	BlockTypeProphetBlock BlockType = 194

	// BlockTypeSignatureBlock is the custom block type code for a SignatureBlock, bpv7/extension_block_signature.go
	BlockTypeSignatureBlock BlockType = 195

	BlockTypeRECJobQuery  BlockType = 1001
	BlockTypeRECNamedData BlockType = 1010
)

// CanonicalBlock represents the canonical bundle block defined in section 4.3.2
type CanonicalBlock struct {
	BlockNumber       uint64
	BlockControlFlags BlockControlFlags
	CRCType           CRCType
	CRC               []byte
	Value             ExtensionBlock
}

// NewCanonicalBlock based on its number, some control flags and an Extension Block.
func NewCanonicalBlock(no uint64, bcf BlockControlFlags, value ExtensionBlock) CanonicalBlock {
	return CanonicalBlock{
		BlockNumber:       no,
		BlockControlFlags: bcf,
		CRCType:           CRCNo,
		CRC:               nil,
		Value:             value,
	}
}

// TypeCode returns the block type code.
func (cb *CanonicalBlock) TypeCode() BlockType {
	return cb.Value.BlockTypeCode()
}

// HasCRC returns if the CRCType indicates a CRC is present for this block.
func (cb *CanonicalBlock) HasCRC() bool {
	return cb.GetCRCType() != CRCNo
}

// GetCRCType returns the CRCType of this block.
func (cb *CanonicalBlock) GetCRCType() CRCType {
	return cb.CRCType
}

// SetCRCType sets the CRC type.
func (cb *CanonicalBlock) SetCRCType(crcType CRCType) {
	cb.CRCType = crcType
}

// MarshalCbor writes this Canonical Block's CBOR representation.
func (cb *CanonicalBlock) MarshalCbor(w io.Writer) error {
	var blockLen uint64 = 5
	if cb.HasCRC() {
		blockLen = 6
	}

	crcBuff := new(bytes.Buffer)
	if cb.HasCRC() {
		w = io.MultiWriter(w, crcBuff)
	}

	if err := cboring.WriteArrayLength(blockLen, w); err != nil {
		return err
	}

	fields := []uint64{uint64(cb.TypeCode()), cb.BlockNumber,
		uint64(cb.BlockControlFlags), uint64(cb.CRCType)}
	for _, f := range fields {
		if err := cboring.WriteUInt(f, w); err != nil {
			return err
		}
	}

	if err := WriteBlock(cb.Value, w); err != nil {
		return fmt.Errorf("marshalling value failed: %v", err)
	}

	if cb.HasCRC() {
		if crcVal, crcErr := calculateCRCBuff(crcBuff, cb.CRCType); crcErr != nil {
			return crcErr
		} else if err := cboring.WriteByteString(crcVal, w); err != nil {
			return err
		} else {
			cb.CRC = crcVal
		}
	}

	return nil
}

// UnmarshalWantedBlock creates this Canonical Block based on a CBOR representation if the type code is in
// wantedBlocks and returns true.
// otherwise it returns false and fills the Value field with a generic block with the correct type code
func (cb *CanonicalBlock) UnmarshalWantedBlock(r io.Reader, wantedBlocks []BlockType) (bool, error) {
	var blockLen uint64
	if bl, err := cboring.ReadArrayLength(r); err != nil {
		return false, err
	} else if bl != 5 && bl != 6 {
		return false, fmt.Errorf("expected array with length 5 or 6, got %d", bl)
	} else {
		blockLen = bl
	}
	old := r
	// Pipe incoming bytes into a separate CRC buffer
	crcBuff := new(bytes.Buffer)
	if blockLen == 6 {
		// Replay array's start
		if err := cboring.WriteArrayLength(blockLen, crcBuff); err != nil {
			return false, err
		}
		r = io.TeeReader(r, crcBuff)
	}

	var blockType BlockType
	if bt, err := cboring.ReadUInt(r); err != nil {
		return false, err
	} else {
		blockType = BlockType(bt)
	}

	if bn, err := cboring.ReadUInt(r); err != nil {
		return false, err
	} else {
		cb.BlockNumber = bn
	}

	if bcf, err := cboring.ReadUInt(r); err != nil {
		return false, err
	} else {
		cb.BlockControlFlags = BlockControlFlags(bcf)
	}

	if crcT, err := cboring.ReadUInt(r); err != nil {
		return false, err
	} else {
		cb.CRCType = CRCType(crcT)
	}

	if !slices.Contains(wantedBlocks, blockType) {
		length, err := cboring.ReadByteStringLen(r)
		if err != nil {
			return false, err
		}
		if s, ok := old.(io.Seeker); ok {
			_, err = s.Seek(int64(length), io.SeekCurrent)
			if err != nil {
				return false, err
			}
		} else {
			_, err = io.CopyN(io.Discard, old, int64(length))
			if err != nil {
				return false, err
			}
		}
		cb.Value = &GenericExtensionBlock{typeCode: blockType}
		if blockLen == 6 {
			if _, err = cboring.ReadByteString(old); err != nil {
				return false, err
			}
		}
		return false, nil
	}

	if b, err := ReadBlock(blockType, r); err != nil {
		return true, fmt.Errorf("unmarshalling block type %d failed: %v", blockType, err)
	} else {
		cb.Value = b
	}

	if blockLen == 6 {
		if crcCalc, crcErr := calculateCRCBuff(crcBuff, cb.CRCType); crcErr != nil {
			return true, crcErr
		} else if crcVal, err := cboring.ReadByteString(r); err != nil {
			return true, err
		} else if !bytes.Equal(crcCalc, crcVal) {
			return true, fmt.Errorf("invalid CRC value: %x instead of expected %x", crcVal, crcCalc)
		} else {
			cb.CRC = crcVal
		}
	}

	return true, nil
}

// UnmarshalCbor creates this Canonical Block based on a CBOR representation.
func (cb *CanonicalBlock) UnmarshalCbor(r io.Reader) error {
	var blockLen uint64
	if bl, err := cboring.ReadArrayLength(r); err != nil {
		return err
	} else if bl != 5 && bl != 6 {
		return fmt.Errorf("expected array with length 5 or 6, got %d", bl)
	} else {
		blockLen = bl
	}

	// Pipe incoming bytes into a separate CRC buffer
	crcBuff := new(bytes.Buffer)
	if blockLen == 6 {
		// Replay array's start
		if err := cboring.WriteArrayLength(blockLen, crcBuff); err != nil {
			return err
		}
		r = io.TeeReader(r, crcBuff)
	}

	var blockType BlockType
	if bt, err := cboring.ReadUInt(r); err != nil {
		return err
	} else {
		blockType = BlockType(bt)
	}

	if bn, err := cboring.ReadUInt(r); err != nil {
		return err
	} else {
		cb.BlockNumber = bn
	}

	if bcf, err := cboring.ReadUInt(r); err != nil {
		return err
	} else {
		cb.BlockControlFlags = BlockControlFlags(bcf)
	}

	if crcT, err := cboring.ReadUInt(r); err != nil {
		return err
	} else {
		cb.CRCType = CRCType(crcT)
	}

	if b, err := ReadBlock(blockType, r); err != nil {
		return fmt.Errorf("unmarshalling block type %d failed: %v", blockType, err)
	} else {
		cb.Value = b
	}

	if blockLen == 6 {
		if crcCalc, crcErr := calculateCRCBuff(crcBuff, cb.CRCType); crcErr != nil {
			return crcErr
		} else if crcVal, err := cboring.ReadByteString(r); err != nil {
			return err
		} else if !bytes.Equal(crcCalc, crcVal) {
			return fmt.Errorf("invalid CRC value: %x instead of expected %x", crcVal, crcCalc)
		} else {
			cb.CRC = crcVal
		}
	}

	return nil
}

// MarshalJSON writes a JSON object for this Canonical Block.
func (cb *CanonicalBlock) MarshalJSON() ([]byte, error) {
	var dataField interface{}

	if _, ok := cb.Value.(json.Marshaler); ok {
		dataField = cb.Value
	} else {
		var buff bytes.Buffer
		if err := WriteBlock(cb.Value, &buff); err != nil {
			return nil, err
		}
		dataField = buff.Bytes()
	}

	return json.Marshal(&struct {
		BlockNumber   uint64            `json:"blockNumber"`
		BlockTypeCode BlockType         `json:"blockTypeCode"`
		BlockType     string            `json:"blockType"`
		ControlFlags  BlockControlFlags `json:"blockControlFlags"`
		Data          interface{}       `json:"data"`
	}{
		BlockNumber:   cb.BlockNumber,
		BlockType:     cb.Value.BlockTypeName(),
		BlockTypeCode: cb.Value.BlockTypeCode(),
		ControlFlags:  cb.BlockControlFlags,
		Data:          dataField,
	})
}

// CheckValid returns an array of errors for incorrect data.
func (cb *CanonicalBlock) CheckValid() (errs error) {
	if bcfErr := cb.BlockControlFlags.CheckValid(); bcfErr != nil {
		errs = multierror.Append(errs, bcfErr)
	}

	if extErr := cb.Value.CheckValid(); extErr != nil {
		errs = multierror.Append(errs, extErr)
	}

	if cb.Value.BlockTypeCode() == BlockTypePayloadBlock && cb.BlockNumber != 1 {
		errs = multierror.Append(errs, fmt.Errorf(
			"CanonicalBlock is a PayloadBlock with a block number %d != 1", cb.BlockNumber))
	}

	return
}

func (cb *CanonicalBlock) String() string {
	var b strings.Builder

	_, _ = fmt.Fprintf(&b, "block type code: %d, ", cb.Value.BlockTypeCode())
	_, _ = fmt.Fprintf(&b, "block number: %d, ", cb.BlockNumber)
	_, _ = fmt.Fprintf(&b, "block processing control flags: %b, ", cb.BlockControlFlags)
	_, _ = fmt.Fprintf(&b, "crc type: %v, ", cb.CRCType)
	_, _ = fmt.Fprintf(&b, "data: %v", cb.Value)

	if cb.HasCRC() {
		_, _ = fmt.Fprintf(&b, ", crc: %x", cb.CRC)
	}

	return b.String()
}
