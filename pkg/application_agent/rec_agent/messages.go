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
	Type MessageType `msgpack:"type"`
}

type Reply struct {
	Message
	Success bool   `msgpack:"success"`
	Error   string `msgpack:"error"`
}

type Register struct {
	Message
	EndpointID string `msgpack:"endpoint_id"`
}

type Fetch struct {
	Message
	EndpointID string           `msgpack:"endpoint_id"`
	NodeType   bpv7.RECNodeType `msgpack:"node_type"`
}

type FetchReply struct {
	Reply
	Bundles []BundleData `msgpack:"bundles"`
}

type BundleCreate struct {
	Message
	Bundle BundleData `msgpack:"bundle"`
}

type BundleType uint8

const (
	BndlTypeJobsQuery BundleType = 1
	BndlTypeJobsReply BundleType = 2
)

type BundleData struct {
	Type        BundleType `msgpack:"type"`
	Source      string     `msgpack:"source"`
	Destination string     `msgpack:"destination"`
	Payload     []byte     `msgpack:"payload"`
	// Used for bpv7.RECJobQueryBlock
	Submitter string `msgpack:"submitter,omitempty"`
}
