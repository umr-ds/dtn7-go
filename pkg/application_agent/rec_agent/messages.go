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

type ControlMessage interface {
	MsgType() MessageType
}

type Message struct {
	Type MessageType
}

func (msg Message) MsgType() MessageType {
	return msg.Type
}

type Reply struct {
	Message
	Success bool
	Error   string
}

func (msg Reply) MsgType() MessageType {
	return msg.Type
}

type Register struct {
	Message
	EID string
}

func (msg Register) MsgType() MessageType {
	return msg.Type
}

type Fetch struct {
	Message
	EID   string
	NType bpv7.RECNodeType
}

func (msg Fetch) MsgType() MessageType {
	return msg.Type
}

type FetchReply struct {
	Reply
	Messages []BndlMessage
}

func (msg FetchReply) MsgType() MessageType {
	return msg.Type
}

type BundleCreate struct {
	Message
	Bndl BndlMessage
}

func (msg BundleCreate) MsgType() MessageType {
	return msg.Type
}

type BundleType uint8

const (
	BndlTypeJobsQuery BundleType = 1
	BndlTypeJobsReply BundleType = 2
)

type BndlMessage interface {
	BndlType() BundleType
}

type BundleMessage struct {
	Type      BundleType
	Sender    string
	Recipient string
}

func (msg BundleMessage) BndlType() BundleType {
	return msg.Type
}

type BundleJobsQuery struct {
	BundleMessage
	Submitter string
}

func (msg BundleJobsQuery) BndlType() BundleType {
	return msg.Type
}

type BundleJobsReply struct {
	BundleMessage
	Queued    []string
	Completed []string
}

func (msg BundleJobsReply) BndlType() BundleType {
	return msg.Type
}
