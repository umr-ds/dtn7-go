// SPDX-FileCopyrightText: 2025, 2026 Markus Sommer
//
// SPDX-License-Identifier: GPL-3.0-or-later

package unix_agent

type MessageType uint8

const (
	MsgTypeGeneralResponse         MessageType = 1
	MsgTypeRegisterEID             MessageType = 2
	MsgTypeUnregisterEID           MessageType = 3
	MsgTypeBundleCreate            MessageType = 4
	MsgTypeBundleCreateResponse    MessageType = 5
	MsgTypeListBundles             MessageType = 6
	MsgTypeListResponse            MessageType = 7
	MsgTypeFetchBundle             MessageType = 8
	MsgTypeFetchBundleResponse     MessageType = 9
	MsgTypeFetchAllBundles         MessageType = 10
	MsgTypeFetchAllBundlesResponse MessageType = 11
)

type Message struct {
	Type MessageType
}

type GeneralResponse struct {
	Message
	Error string
}

type RegisterUnregisterMessage struct {
	Message
	EndpointID string
}

type BundleCreateMessage struct {
	Message
	Args map[string]interface{}
}

type BundleCreateResponse struct {
	GeneralResponse
	BundleID string
}

type ListBundles struct {
	Message
	Mailbox string
	New     bool
}

type ListResponse struct {
	GeneralResponse
	Bundles []string
}

type FetchBundle struct {
	Message
	Mailbox  string
	BundleID string
	Remove   bool
}

type BundleContent struct {
	BundleID      string
	SourceID      string
	DestinationID string
	Payload       []byte
}

type FetchBundleResponse struct {
	GeneralResponse
	BundleContent BundleContent
}

type FetchAllBundles struct {
	Message
	Mailbox string
	New     bool
	Remove  bool
}

type FetchAllBundlesResponse struct {
	GeneralResponse
	Bundles []BundleContent
}
