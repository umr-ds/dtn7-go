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

type RECJobQueryBlock struct {
	Submitter string
}

func NewRECJobQueryBlock(Submitter string) *RECJobQueryBlock {
	query := RECJobQueryBlock{Submitter: Submitter}
	return &query
}

func (block *RECJobQueryBlock) CheckValid() error {
	return nil
}

func (block *RECJobQueryBlock) BlockTypeCode() uint64 {
	return BlockTypeRECJobQuery
}

func (block *RECJobQueryBlock) BlockTypeName() string {
	return "REC Job Query"
}

func (block *RECJobQueryBlock) CheckContextValid(*Bundle) error {
	// TODO: check if multiple rec blocks exist
	return nil
}

func (block *RECJobQueryBlock) MarshalCbor(w io.Writer) error {
	if err := cboring.WriteArrayLength(1, w); err != nil {
		return err
	}

	if err := cboring.WriteTextString(block.Submitter, w); err != nil {
		return err
	}

	return nil
}

func (block *RECJobQueryBlock) UnmarshalCbor(r io.Reader) error {
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

type RECNamedDataAction uint64

const (
	RECNamedDataActionPut    RECNamedDataAction = 1
	RECNamedDataActionGet    RECNamedDataAction = 2
	RECNamedDataActionDelete RECNamedDataAction = 3
)

type RECNamedDataBlock struct {
	Action RECNamedDataAction
	Name   string
}

func NewRECNamedDataBlock(action RECNamedDataAction, name string) *RECNamedDataBlock {
	block := RECNamedDataBlock{
		Action: action,
		Name:   name,
	}
	return &block
}

func (block *RECNamedDataBlock) CheckValid() error {
	return nil
}

func (block *RECNamedDataBlock) BlockTypeCode() uint64 {
	return BlockTypeRECNamedData
}

func (block *RECNamedDataBlock) BlockTypeName() string {
	return "REC Named Data"
}

func (block *RECNamedDataBlock) CheckContextValid(*Bundle) error {
	// TODO: check if multiple rec blocks exist
	return nil
}

func (block *RECNamedDataBlock) MarshalCbor(w io.Writer) error {
	if err := cboring.WriteArrayLength(2, w); err != nil {
		return err
	}

	if err := cboring.WriteUInt(uint64(block.Action), w); err != nil {
		return err
	}

	if err := cboring.WriteTextString(block.Name, w); err != nil {
		return err
	}

	return nil
}

func (block *RECNamedDataBlock) UnmarshalCbor(r io.Reader) error {
	if l, err := cboring.ReadArrayLength(r); err != nil {
		return err
	} else if l != 2 {
		return fmt.Errorf("expected 2 field, got %d", l)
	}

	action, err := cboring.ReadUInt(r)
	if err != nil {
		return err
	}
	block.Action = RECNamedDataAction(action)

	name, err := cboring.ReadTextString(r)
	if err != nil {
		return err
	}

	block.Name = name

	return nil
}
