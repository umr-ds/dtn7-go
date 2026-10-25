// SPDX-FileCopyrightText: 2025, 2026 Markus Sommer
//
// SPDX-License-Identifier: GPL-3.0-or-later

package application_agent

import (
	"fmt"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
)

type AgentAlreadyRegisteredError string

func NewAgentAlreadyRegisteredError(name string) error {
	return AgentAlreadyRegisteredError(name)
}

func (err AgentAlreadyRegisteredError) Error() string {
	return fmt.Sprintf("Agent has already been registered: %v", string(err))
}

type NoSuchAgentError string

func NewNoSuchAgentError(name string) error {
	return NoSuchAgentError(name)
}

func (err NoSuchAgentError) Error() string {
	return fmt.Sprintf("No such agent registered: %v", string(err))
}

type IDAlreadyRegisteredError bpv7.EndpointID

func NewIDAlreadyRegisteredError(eid bpv7.EndpointID) error {
	return IDAlreadyRegisteredError(eid)
}

func (err IDAlreadyRegisteredError) Error() string {
	return fmt.Sprintf("ID has already been registered: %v", bpv7.EndpointID(err).String())
}

type NoSuchIDError bpv7.EndpointID

func NewNoSuchIDError(eid bpv7.EndpointID) error {
	return NoSuchIDError(eid)
}

func (err NoSuchIDError) Error() string {
	return fmt.Sprintf("No such ID has been registered: %v", bpv7.EndpointID(err).String())
}

type AlreadyDeliveredError bpv7.BundleID

func NewAlreadyDeliveredError(bid bpv7.BundleID) error {
	return AlreadyDeliveredError(bid)
}

func (err AlreadyDeliveredError) Error() string {
	return fmt.Sprintf("Bundle %v already in mailbox", bpv7.BundleID(err).String())
}

type NoSuchBundleError bpv7.BundleID

func NewNoSuchBundleError(bid bpv7.BundleID) error {
	return NoSuchBundleError(bid)
}

func (err NoSuchBundleError) Error() string {
	return fmt.Sprintf("No Bundle with id %v in mailbox", bpv7.BundleID(err).String())
}
