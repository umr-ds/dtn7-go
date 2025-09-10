package bpv7

import (
	"fmt"
	"io"

	"github.com/dtn7/cboring"
)

type RECNodeType uint8

const (
	NTypeBroker    RECNodeType = 1
	NTypeExecutor  RECNodeType = 2
	NTypeDataStore RECNodeType = 3
	NTypeClient    RECNodeType = 4
)

type RECJobQuery struct {
	Submitter string
}

func NewRECJobQueryBlock(Submitter string) *RECJobQuery {
	query := RECJobQuery{Submitter: Submitter}
	return &query
}

func (block *RECJobQuery) CheckValid() error {
	return nil
}

func (block *RECJobQuery) BlockTypeCode() uint64 {
	return BlockTypeRECJobQuery
}

func (block *RECJobQuery) BlockTypeName() string {
	return "REC Job Query"
}

func (block *RECJobQuery) CheckContextValid(*Bundle) error {
	// TODO: check if multiple rec blocks exist
	return nil
}

func (block *RECJobQuery) MarshalCbor(w io.Writer) error {
	if err := cboring.WriteArrayLength(1, w); err != nil {
		return err
	}

	if err := cboring.WriteTextString(block.Submitter, w); err != nil {
		return err
	}

	return nil
}

func (block *RECJobQuery) UnmarshalCbor(r io.Reader) error {
	if l, err := cboring.ReadArrayLength(r); err != nil {
		return err
	} else if l != 1 {
		return fmt.Errorf("expected 1 field, got %d", l)
	}

	submitter, err := cboring.ReadTextString(r)
	if err != nil {
		return err
	}

	block.Submitter = submitter

	return nil
}
