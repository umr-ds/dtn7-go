// SPDX-FileCopyrightText: 2019, 2020, 2022 Alvar Penning
// SPDX-FileCopyrightText: 2020, 2021, 2022 Matthias Axel Kröll
// SPDX-FileCopyrightText: 2021 Markus Sommer
//
// SPDX-License-Identifier: GPL-3.0-or-later

package bpv7

import (
	"bytes"
	"encoding"
	"fmt"
	"io"

	"github.com/dtn7/cboring"
)

// ExtensionBlock describes the block-type specific data of any Canonical Block.
//
// Such an ExtensionBlock must implement either the cboring.CborMarshaler interface, if its serializable
// to / from CBOR, or both encoding.BinaryMarshaler and encoding.BinaryUnmarshaler. The latter allows any kind
// of serialization, e.g., to a totally custom format.
//
// Furthermore, an ExtensionBlock can implement the json.Marshaler for a more human-readable representation.
type ExtensionBlock interface {
	Valid

	// CheckContextValid performs a self-check like CheckValid, but also passing
	// a reference to the surrounding Bundle, allowing, e.g., uniqueness checks.
	CheckContextValid(*Bundle) error

	// BlockTypeCode must return a constant integer, indicating the block type code.
	BlockTypeCode() BlockType

	// BlockTypeName must return a constant string, this block's name.
	BlockTypeName() string
}

// createBlock returns either a specific ExtensionBlock or, if type code is unknown, an GenericExtensionBlock.
func createBlock(typeCode BlockType) ExtensionBlock {
	switch typeCode {
	case BlockTypePayloadBlock:
		return &PayloadBlock{}
	case BlockTypePreviousNodeBlock:
		return &PreviousNodeBlock{}
	case BlockTypeBundleAgeBlock:
		b := BundleAgeBlock(0)
		return &b
	case BlockTypeHopCountBlock:
		return &HopCountBlock{}
	case BlockTypeBinarySprayBlock:
		b := BinarySprayBlock(0)
		return &b
	case BlockTypeSignatureBlock:
		return &SignatureBlock{}
	case BlockTypeRECBundleType:
		b := RECBundleTypeBlock(0)
		return &b
	default:
		return &GenericExtensionBlock{typeCode: typeCode}
	}
}

// WriteBlock writes an ExtensionBlock in its correct binary format into the io.Writer.
// Unknown block types are treated as GenericExtensionBlock.
func WriteBlock(b ExtensionBlock, w io.Writer) error {
	switch b := b.(type) {
	case encoding.BinaryMarshaler:
		if data, err := b.MarshalBinary(); err != nil {
			return fmt.Errorf("marshalling binary for Block erred: %v", err)
		} else {
			return cboring.WriteByteString(data, w)
		}

	case cboring.CborMarshaler:
		var buff bytes.Buffer
		if err := cboring.Marshal(b, &buff); err != nil {
			return fmt.Errorf("marshalling CBOR for Block erred: %v", err)
		}
		return cboring.WriteByteString(buff.Bytes(), w)

	default:
		return fmt.Errorf("ExtensionBlockByType does not implement any expected types")
	}
}

// ReadBlock reads an ExtensionBlock from its correct binary format from the io.Reader.
// Unknown block types are treated as GenericExtensionBlock.
func ReadBlock(typeCode BlockType, r io.Reader) (b ExtensionBlock, err error) {
	b = createBlock(typeCode)

	switch b := b.(type) {
	case encoding.BinaryUnmarshaler:
		if data, dataErr := cboring.ReadByteString(r); dataErr != nil {
			err = dataErr
		} else {
			err = b.UnmarshalBinary(data)
		}

	case cboring.CborMarshaler:
		if data, dataErr := cboring.ReadByteString(r); dataErr != nil {
			err = dataErr
		} else {
			var buff = bytes.NewBuffer(data)
			err = cboring.Unmarshal(b, buff)
		}

	default:
		err = fmt.Errorf("ExtensionBlockByType does not implement any expected types")
	}

	return
}
