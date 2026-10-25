// SPDX-FileCopyrightText: 2019, 2020 Alvar Penning
// SPDX-FileCopyrightText: 2025, 2026 Markus Sommer
//
// SPDX-License-Identifier: GPL-3.0-or-later

package bpv7

import (
	"bytes"
	"reflect"
	"testing"
)

func TestExtensionBlockManagerRWBlock(t *testing.T) {

	tests := []struct {
		from     ExtensionBlock
		to       []byte
		typeCode BlockType
	}{
		// CBOR; wrapped within a CBOR byte string
		{NewBundleAgeBlock(23), []byte{0x41, 0x17}, BlockTypeBundleAgeBlock},
		{NewHopCountBlock(16), []byte{0x43, 0x82, 0x10, 0x00}, BlockTypeHopCountBlock},
		{NewPreviousNodeBlock(MustNewEndpointID("dtn://23/")), []byte{0x48, 0x82, 0x01, 0x65, 0x2F, 0x2F, 0x32, 0x33, 0x2F}, BlockTypePreviousNodeBlock},
		{NewBinarySprayBlock(15), []byte{0x41, 0x0f}, BlockTypeBinarySprayBlock},

		// Binary; also wrapped, of course
		{NewGenericExtensionBlock([]byte{0xFF}, 8080), []byte{0x41, 0xFF}, 8080},
		{NewPayloadBlock([]byte("lel")), []byte{0x43, 0x6C, 0x65, 0x6C}, BlockTypePayloadBlock},
	}

	for _, test := range tests {
		// Block -> Binary / CBOR
		var buff = new(bytes.Buffer)
		if err := WriteBlock(test.from, buff); err != nil {
			t.Fatal(err)
		} else if to := buff.Bytes(); !bytes.Equal(to, test.to) {
			t.Fatalf("Bytes are not equal: %x != %x", test.to, to)
		}

		// Binary / CBOR -> Block
		buff = bytes.NewBuffer(test.to)
		if b, err := ReadBlock(test.typeCode, buff); err != nil {
			t.Fatal(err)
		} else if !reflect.DeepEqual(b, test.from) {
			t.Fatalf("Blocks differ: %v %v", test.from, b)
		}
	}
}
