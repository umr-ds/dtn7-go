// SPDX-FileCopyrightText: 2019, 2020 Alvar Penning
// SPDX-FileCopyrightText: 2024 Markus Sommer
//
// SPDX-License-Identifier: GPL-3.0-or-later

package mtcp

import (
	"fmt"
	"sync"
	"testing"

	"pgregory.net/rapid"

	"github.com/dtn7/dtn7-go/internal/testhelpers"
	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
)

func setup() *cla.Manager {
	receive := func(bundle *bpv7.Bundle) {}
	connect := func(eid bpv7.EndpointID) {}
	disconnect := func(eid bpv7.EndpointID) {}

	manager := cla.NewCLAManager(receive, connect, disconnect)
	cla.InitialiseCLAManager(manager)
	return manager
}

func teardown(manager *cla.Manager) {
	manager.Shutdown()
}

func TestSendReceive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		manager := setup()
		defer teardown(manager)

		port := testhelpers.RandomPort(t, "tcp")
		numberOfClients := rapid.IntRange(1, 25).Draw(t, "Number of Clients")
		numberOfBundles := uint8(rapid.IntRange(1, 256).Draw(t, "Number of Bundles"))
		var wgSend sync.WaitGroup
		wgSend.Add(int(numberOfBundles))
		var wgReceive sync.WaitGroup
		wgReceive.Add(int(numberOfBundles))

		bundles := make([]*bpv7.Bundle, numberOfBundles)
		var i uint8
		for i = 0; i < numberOfBundles; i++ {
			bundles[i] = testhelpers.RandomBundle.Draw(t, fmt.Sprintf("bundle %d", i))
		}

		receiveFunc := func(bundle *bpv7.Bundle) {
			wgReceive.Done()
		}

		// Server
		serv := NewMTCPServer(
			fmt.Sprintf(":%d", port), bpv7.MustNewEndpointID("dtn://mtcpcla/"), receiveFunc)
		if err := serv.Start(); err != nil {
			t.Fatal(err)
		}

		clients := make([]*MTCPClient, numberOfClients)
		for i := range numberOfClients {
			client := NewAnonymousMTCPClient(fmt.Sprintf("localhost:%d", port))
			if err := client.Activate(); err != nil {
				t.Fatal(fmt.Errorf("starting Client failed: %v", err))
			}
			clients[i] = client
		}

		for i = range numberOfBundles {
			sender := clients[rapid.IntRange(0, len(clients)-1).Draw(t, fmt.Sprintf("Sender %v", i))]
			go func(i uint8, sender *MTCPClient) {
				bundle := bundles[i]
				err := sender.Send(bundle)
				wgSend.Done()
				if err != nil {
					t.Fatal(err)
				}
			}(i, sender)
		}
		wgSend.Wait()
		wgReceive.Wait()

		for _, client := range clients {
			err := client.Close()
			if err != nil {
				t.Fatal(err)
			}
		}

		err := serv.Close()
		if err != nil {
			t.Fatal(err)
		}
	})
}
