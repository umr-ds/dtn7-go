package store

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	log "github.com/sirupsen/logrus"
	"pgregory.net/rapid"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
)

func initTest(t *rapid.T) {
	log.SetLevel(log.ErrorLevel)
	nodeID, err := bpv7.NewEndpointID(rapid.StringMatching(bpv7.DtnEndpointRegexpNotNone).Draw(t, "nodeID"))
	if err != nil {
		t.Fatal(err)
	}

	err = InitialiseStore(nodeID, "/tmp/dtn7-test")
	if err != nil {
		t.Fatal(err)
	}
}

func cleanupTest(t *rapid.T) {
	err := ShutdownStore()
	if err != nil {
		t.Fatal(err)
	}
	err = os.RemoveAll("/tmp/dtn7-test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestBundleInsertion(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		initTest(t)
		defer cleanupTest(t)

		bundle := bpv7.GenerateRandomizedBundle(t, 0)
		bd, err := GetStoreSingleton().insertNewBundle(bundle)
		if err != nil {
			t.Fatal(err)
		}

		bdLoad, err := GetStoreSingleton().GetBundleDescriptor(bundle.ID())
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(bd, bdLoad) {
			t.Fatal("Retrieved BundleDescriptor not equal")
		}

		bundleLoad, err := bdLoad.Load()
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(bundle, bundleLoad) {
			t.Fatal("Retrieved Bundle not equal")
		}
	})
}

func TestConstraints(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		initTest(t)
		defer cleanupTest(t)

		bundle := bpv7.GenerateRandomizedBundle(t, 0)
		bd, err := GetStoreSingleton().insertNewBundle(bundle)
		if err != nil {
			t.Fatal(err)
		}

		numConstraints := rapid.IntRange(1, 5).Draw(t, "Number of constraints")
		constraints := make([]Constraint, numConstraints)
		for i := range constraints {
			constraint := Constraint(rapid.IntRange(int(DispatchPending), int(ReassemblyPending)).Draw(t, fmt.Sprintf("constraint %v", i)))
			constraints[i] = constraint
		}

		// test constraint addition
		addConstraints(t, bd, constraints)
		// test constraint deletion
		removeConstraints(t, bd, constraints)

		// test constraint reset
		addConstraints(t, bd, constraints)
		err = bd.ResetConstraints()
		if err != nil {
			t.Fatalf("Error resetting constraints: %v", err)
		}
		if bd.Retain() || len(bd.retentionConstraints) > 0 {
			t.Fatal("RetentionConstraint reset failed")
		}
	})
}

func addConstraints(t *rapid.T, bd *BundleDescriptor, constraints []Constraint) {
	for _, constraint := range constraints {
		err := bd.AddConstraint(constraint)
		if err != nil {
			t.Fatal(err)
		}
		if !(len(bd.retentionConstraints) > 0) {
			t.Fatal("Retention constraints empty after addition")
		}
		if !bd.Retain() {
			t.Fatal("Retention-flag not set after addition")
		}
		if !(bd.retentionConstraints[len(bd.retentionConstraints)-1] == constraint) {
			t.Fatalf("Constraint %v not in descriptor constraints %v", constraint, bd.retentionConstraints)
		}
	}
}

func removeConstraints(t *rapid.T, bd *BundleDescriptor, constraints []Constraint) {
	for _, constraint := range constraints {
		err := bd.RemoveConstraint(constraint)
		if err != nil {
			t.Fatalf("Error removing constraint: %v", err)
		}

		if (len(bd.retentionConstraints) == 0) && bd.Retain() {
			t.Fatal("Retention flag still set after all constraints removed")
		}

		for _, conLoad := range bd.retentionConstraints {
			if conLoad == constraint {
				t.Fatalf("Constraint %v still present after deletion: %v", constraint, bd.retentionConstraints)
			}
		}
	}
}
