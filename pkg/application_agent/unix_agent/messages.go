// SPDX-FileCopyrightText: 2025 Markus Sommer
//
// SPDX-License-Identifier: GPL-3.0-or-later

package unix_agent

type MessageType uint8

const (
	MsgTypeGeneralResponse       MessageType = 1
	MsgTypeRegisterEID           MessageType = 2
	MsgTypeUnregisterEID         MessageType = 3
	MsgTypeBundleCreate          MessageType = 4
	MsgTypeBundleCreateResponse  MessageType = 5
	MsgTypeListBundles           MessageType = 6
	MsgTypeListResponse          MessageType = 7
	MsgTypeGetBundle             MessageType = 8
	MsgTypeGetBundleResponse     MessageType = 9
	MsgTypeGetAllBundles         MessageType = 10
	MsgTypeGetAllBundlesResponse MessageType = 11
)

type Message struct {
	Type MessageType
}

type GeneralResponse struct {
	Message
	Success bool
	Error   string
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
	Message
	Error    string
	BundleID string
}

type MailboxListMessage struct {
	Message
	Mailbox string
	New     bool
}

type MailboxListResponse struct {
	GeneralResponse
	Bundles []string
}

type GetBundleMessage struct {
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

type GetBundleResponse struct {
	GeneralResponse
	BundleContent
}

type GetAllBundlesMessage struct {
	Message
	Mailbox string
	New     bool
	Remove  bool
}

type GetAllBundlesResponse struct {
	GeneralResponse
	Bundles []BundleContent
}
