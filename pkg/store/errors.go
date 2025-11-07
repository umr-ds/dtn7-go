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
