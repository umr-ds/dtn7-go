package rec_agent

import "github.com/dtn7/dtn7-go/pkg/bpv7"

type MsgType uint8

const (
	MsgTypeReply      MsgType = 1
	MsgTypeRegister   MsgType = 2
	MsgTypeFetch      MsgType = 3
	MsgTypeFetchReply MsgType = 4
	MsgTypeJobsQuery  MsgType = 5
	MsgTypeJobsReply  MsgType = 6
)

type MsgStatus uint8

const (
	MsgStatusSuccess MsgStatus = 1
	MsgStatusFailure MsgStatus = 2
)

type Message struct {
	Type MsgType
}

type Reply struct {
	Message
	Status MsgStatus
	Text   string
}

type Register struct {
	Message
	EID   string
	NType bpv7.RECNodeType
}

type Fetch struct {
	Message
	EID   string
	NType bpv7.RECNodeType
}

type FetchReply struct {
	Reply
	Messages []BundleMessage
}

type BundleMessage struct {
	Message
	Sender    string
	Recipient string
}

type BundleReply struct {
	Reply
	Sender    string
	Recipient string
}

type JobsQuery struct {
	BundleMessage
	Submitter string
}

type JobsReply struct {
	BundleReply
	Queued    []string
	Completed []string
}
