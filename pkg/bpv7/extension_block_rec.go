package bpv7

import (
	"fmt"
	"io"

	"github.com/dtn7/cboring"
)

type RECBundleTypeBlock uint64

func NewRECBundleTypeBlock(btype uint8) *RECBundleTypeBlock {
	blck := RECBundleTypeBlock(btype)
	return &blck
}

func (block *RECBundleTypeBlock) CheckValid() error {
	return nil
}

func (block *RECBundleTypeBlock) BlockTypeCode() uint64 {
	return BlockTypeRECBundleType
}

func (block *RECBundleTypeBlock) BlockTypeName() string {
	return "REC Block Type"
}

func (block *RECBundleTypeBlock) CheckContextValid(*Bundle) error {
	// TODO: check if multiple rec blocks exist
	return nil
}

func (block *RECBundleTypeBlock) MarshalCbor(w io.Writer) error {
	if err := cboring.WriteArrayLength(1, w); err != nil {
		return err
	}

	if err := cboring.WriteUInt(uint64(*block), w); err != nil {
		return err
	}

	return nil
}

func (block *RECBundleTypeBlock) UnmarshalCbor(r io.Reader) error {
	if l, err := cboring.ReadArrayLength(r); err != nil {
		return err
	} else if l != 1 {
		return fmt.Errorf("expected 1 field, got %d", l)
	}

	btype, err := cboring.ReadUInt(r)
	if err != nil {
		return err
	}

	*block = RECBundleTypeBlock(btype)

	return nil
}
