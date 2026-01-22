package store

import (
	"fmt"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
)

type NoSuchBundleError bpv7.BundleID

func NewNoSuchBundleError(bid bpv7.BundleID) *NoSuchBundleError {
	err := NoSuchBundleError(bid)
	return &err
}

func (err *NoSuchBundleError) Error() string {
	return fmt.Sprintf("No bundle with id %v in store", bpv7.BundleID(*err))
}

type BundleDeletedError bpv7.BundleID

func NewBundleDeletedError(bid bpv7.BundleID) *BundleDeletedError {
	err := BundleDeletedError(bid)
	return &err
}

func (err *BundleDeletedError) Error() string {
	return fmt.Sprintf("Bundle %v has been deleted", bpv7.BundleID(*err))
}

type InvalidConstraint Constraint

func NewInvalidConstraint(constraint Constraint) *InvalidConstraint {
	ic := InvalidConstraint(constraint)
	return &ic
}

func (ic *InvalidConstraint) Error() string {
	return fmt.Sprintf("%v is not a valid retention constraint", int(*ic))
}

type HasConstraintsError []Constraint

func NewHasConstraintsError(constraints []Constraint) *HasConstraintsError {
	ce := HasConstraintsError(constraints)
	return &ce
}

func (ce *HasConstraintsError) Error() string {
	return fmt.Sprintf("Bundle still has constraints: %v", []Constraint(*ce))
}
