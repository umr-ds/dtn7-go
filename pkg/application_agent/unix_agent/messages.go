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
	MsgTypeList                  MessageType = 5
	MsgTypeListResponse          MessageType = 6
	MsgTypeGetBundle             MessageType = 7
	MsgTypeGetBundleResponse     MessageType = 8
	MsgTypeGetAllBundles         MessageType = 9
	MsgTypeGetAllBundlesResponse MessageType = 10
)

type Message struct {
	Type MessageType `msgpack:"type"`
}

type GeneralResponse struct {
	Message
	Success bool   `msgpack:"success"`
	Error   string `msgpack:"error"`
}

type RegisterUnregisterMessage struct {
	Message
	EndpointID string `msgpack:"endpoint_id"`
}

type BundleCreateMessage struct {
	Message
	DestinationID     *string `msgpack:"destination_id"`
	Payload           []byte  `msgpack:"payload"`
	SourceID          *string `msgpack:"source_id,omitempty"`
	CreationTimestamp *string `msgpack:"creation_timestamp,omitempty"`
	Lifetime          *string `msgpack:"lifetime,omitempty"`
	ReportTo          *string `msgpack:"report_to,omitempty"`
	BundleFlags       *uint32 `msgpack:"bundle_flags,omitempty"`
	BlockFlags        *uint32 `msgpack:"block_flags,omitempty"`
}

type MailboxListMessage struct {
	Message
	Mailbox string `msgpack:"mailbox"`
	New     bool   `msgpack:"new"`
}

type MailboxListResponse struct {
	GeneralResponse
	Bundles []string `msgpack:"bundles"`
}

type GetBundleMessage struct {
	Message
	Mailbox  string `msgpack:"mailbox"`
	BundleID string `msgpack:"bundle_id"`
	Remove   bool   `msgpack:"remove"`
}

type BundleContent struct {
	BundleID      string `msgpack:"bundle_id"`
	SourceID      string `msgpack:"source_id"`
	DestinationID string `msgpack:"destination_id"`
	Payload       []byte `msgpack:"payload"`
}

type GetBundleResponse struct {
	GeneralResponse
	BundleContent
}

type GetAllBundlesMessage struct {
	Message
	Mailbox string `msgpack:"mailbox"`
	New     bool   `msgpack:"new"`
	Remove  bool   `msgpack:"remove"`
}

type GetAllBundlesResponse struct {
	GeneralResponse
	Bundles []BundleContent `msgpack:"bundles"`
}
