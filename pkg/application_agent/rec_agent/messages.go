package rec_agent

import "github.com/dtn7/dtn7-go/pkg/bpv7"

type MessageType uint8

const (
	MsgTypeReply        MessageType = 1
	MsgTypeRegister     MessageType = 2
	MsgTypeFetch        MessageType = 3
	MsgTypeFetchReply   MessageType = 4
	MsgTypeBundleCreate MessageType = 5
)

type Message struct {
	Type MessageType
}

type Reply struct {
	Message
	Success bool
	Error   string
}

type Register struct {
	Message
	EndpointID string
}

type Fetch struct {
	Message
	EndpointID string
	NodeType   bpv7.RECNodeType
}

type FetchReply struct {
	Reply
	Bundles []BundleData
}

type BundleCreate struct {
	Message
	Bundle BundleData
}

type BundleType uint8

const (
	BndlTypeJobsQuery BundleType = 1
	BndlTypeJobsReply BundleType = 2
)

type BundleData struct {
	Type        BundleType
	Source      string
	Destination string
	Payload     []byte
	Metadata    map[string]string
}
