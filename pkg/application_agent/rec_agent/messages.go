package rec_agent

type RECNodeType uint8

const (
	NTypeBroker    RECNodeType = 1
	NTypeExecutor  RECNodeType = 2
	NTypeDataStore RECNodeType = 3
	NTypeClient    RECNodeType = 4
)

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
	EndpointID string      `msgpack:"endpoint_id"`
	NodeType   RECNodeType `msgpack:"node_type"`
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

type BundleData struct {
	Type        BundleType `msgpack:"type"`
	Source      string     `msgpack:"source"`
	Destination string     `msgpack:"destination"`
	Payload     []byte     `msgpack:"payload"`
	Success     bool       `msgpack:"success"`
	Error       string     `msgpack:"error"`
	// used by broker discovery
	NodeType RECNodeType `msgpack:"node_type,omitempty"`
	// used by job query/list
	Submitter string `msgpack:"submitter,omitempty"`
	// used by named data
	NamedData string `msgpack:"named_data,omitempty"`
}
