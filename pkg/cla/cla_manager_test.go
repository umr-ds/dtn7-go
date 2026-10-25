// SPDX-FileCopyrightText: 2023, 2024, 2025, 2026 Markus Sommer
//
// SPDX-License-Identifier: GPL-3.0-or-later

package cla

import (
	"fmt"
	"testing"
	"time"

	"github.com/dtn7/dtn7-go/internal/testhelpers"
	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla/dummy_cla"
	log "github.com/sirupsen/logrus"
	"pgregory.net/rapid"
)

func setup() *Manager {
	receive := func(bundle *bpv7.Bundle) {}
	connect := func(eid bpv7.EndpointID) {}
	disconnect := func(eid bpv7.EndpointID) {}

	manager := NewCLAManager(receive, connect, disconnect)
	return manager
}

func teardown(manager *Manager) {
	manager.Shutdown()
}

func TestRegisterListener(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		manager := setup()
		defer teardown(manager)

		numberOfListeners := rapid.Uint8Max(10).Draw(t, "Number of Listeners")
		listeners := make([]*dummy_cla.DummyListener, numberOfListeners)
		for i := range listeners {
			address := rapid.String().Draw(t, fmt.Sprintf("Listener %v address", i))
			listeners[i] = dummy_cla.NewDummyListener(address)
		}

		for _, listener := range listeners {
			err := manager.RegisterListener(listener)
			if err != nil {
				t.Fatal(fmt.Sprintf("Error registering listener %v: %v", listener.Address(), err))
			}
		}

		initialisedListeners := manager.GetListeners()
		for _, listener := range listeners {
			present := false
			for _, initialized := range initialisedListeners {
				if initialized.Address() == listener.Address() && initialized.Running() {
					present = true
					break
				}
			}
			if !present {
				t.Fatal(fmt.Sprintf("Listener with address %v not present", listener.Address()))
			}
		}
	})
}

func TestRegisterSender(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		manager := setup()
		defer teardown(manager)

		numberOfCLAs := rapid.Uint8Max(100).Draw(t, "Number of CLAs")
		clas := make([]Convergence, numberOfCLAs)

		noop := func(bundle bpv7.Bundle) (any, error) {
			return nil, nil
		}

		for i := range clas {
			eid := testhelpers.RandomEndpointID.Draw(t, fmt.Sprintf("CLA %v", i))
			cla, _ := dummy_cla.NewDummyCLAPair(eid, eid, noop)
			clas[i] = cla
			manager.Register(cla)
		}

		time.Sleep(time.Millisecond * 10)

		for _, cla := range clas {
			senders := manager.GetSenders()
			present := false
			for _, sender := range senders {
				if sender.Address() == cla.Address() {
					if sender.Active() {
						present = true
					} else {
						log.Fatalf("CLA %v not activated", cla.Address())
					}
				}
			}
			if !present {
				log.Fatalf("CLA %v not present", cla.Address())
			}
		}
	})
}
