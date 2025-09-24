package rec_agent

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"slices"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/dtn7/dtn7-go/pkg/application_agent"
	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/store"
)

const RECBrokerMulticastAddress = "dtn://rec.broker/~"
const RECDataStoreMulticastAddress = "dtn://rec.store/~"
const RECExecutorMulticastAddress = "dtn://rec.executor/~"
const RECClientMulticastAddress = "dtn://rec.client/~"

type RECAgent struct {
	listenAddress      *net.UnixAddr
	listener           *net.UnixListener
	mailboxes          *application_agent.MailboxBank
	stopChan           chan interface{}
	multicastAddresses map[RECNodeType]bpv7.EndpointID
}

func NewRECAgent(listenAddress string) (*RECAgent, error) {
	unixAddr, err := net.ResolveUnixAddr("unix", listenAddress)
	if err != nil {
		return nil, err
	}

	brokerAddr := bpv7.MustNewEndpointID(RECBrokerMulticastAddress)
	storeAddr := bpv7.MustNewEndpointID(RECDataStoreMulticastAddress)
	executorAddress := bpv7.MustNewEndpointID(RECExecutorMulticastAddress)
	clientAddress := bpv7.MustNewEndpointID(RECClientMulticastAddress)

	multicastAddresses := make(map[RECNodeType]bpv7.EndpointID, 4)
	multicastAddresses[NTypeBroker] = brokerAddr
	multicastAddresses[NTypeExecutor] = executorAddress
	multicastAddresses[NTypeDataStore] = storeAddr
	multicastAddresses[NTypeClient] = clientAddress

	agent := RECAgent{
		listenAddress:      unixAddr,
		mailboxes:          application_agent.NewMailboxBank(),
		stopChan:           make(chan interface{}),
		multicastAddresses: multicastAddresses,
	}

	_ = agent.mailboxes.Register(brokerAddr)
	_ = agent.mailboxes.Register(storeAddr)
	_ = agent.mailboxes.Register(executorAddress)
	_ = agent.mailboxes.Register(clientAddress)

	return &agent, nil
}

func (agent *RECAgent) Name() string {
	return fmt.Sprintf("RECAgent(%v)", agent.listenAddress)
}

func (agent *RECAgent) Start() error {
	log.WithFields(log.Fields{
		"address": agent.listenAddress,
	}).Info("Starting RECAgent")

	listener, err := net.ListenUnix("unix", agent.listenAddress)
	if err != nil {
		return err
	}
	agent.listener = listener

	go agent.listen()

	return nil
}

func (agent *RECAgent) Shutdown() {
	log.WithField("listenAddress", agent.listenAddress).Info("Shutting RECAgent down")
	close(agent.stopChan)
	f, _ := agent.listener.File()
	_ = f.Close()
	_ = agent.listener.Close()
}

func (agent *RECAgent) GC() {
	log.WithField("agent", agent.Name()).Debug("Performing gc")
	agent.mailboxes.GC()
}

func (agent *RECAgent) Endpoints() []bpv7.EndpointID {
	return agent.mailboxes.RegisteredIDs()
}

func (agent *RECAgent) Deliver(bundleDescriptor *store.BundleDescriptor) error {
	return agent.mailboxes.Deliver(bundleDescriptor)
}

func (agent *RECAgent) listen() {
	defer func() {
		log.WithField("listenAddress", agent.listenAddress).Info("Cleaning up socket")
	}()

	for {
		select {
		case <-agent.stopChan:
			return

		default:
			if err := agent.listener.SetDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
				log.WithFields(log.Fields{
					"listener": agent.listener,
					"error":    err,
				}).Error("RECAgent failed to set deadline on UNIX socket")

				agent.Shutdown()
			} else if conn, err := agent.listener.Accept(); err == nil {
				go agent.handleConnection(conn)
			}
		}
	}
}

func (agent *RECAgent) handleConnection(conn net.Conn) {
	connReader := bufio.NewReader(conn)
	connWriter := bufio.NewWriter(conn)

	msgLenBytes := make([]byte, 8)
	log.Debug("Receiving message length")
	_, err := io.ReadFull(connReader, msgLenBytes)
	if err != nil {
		log.WithField("error", err).Error("Failed reading 8-byte message length")
		return
	}

	msgLen := binary.BigEndian.Uint64(msgLenBytes)
	log.WithField("msgLength", msgLen).Debug("Received msgLength")

	log.Debug("Receiving message")
	msgBytes := make([]byte, msgLen)
	_, err = io.ReadFull(connReader, msgBytes)
	if err != nil {
		log.WithField("error", err).Error("Failed reading message")
		return
	}

	log.Debug("Unmarshaling message")
	message := Message{}
	err = msgpack.Unmarshal(msgBytes, &message)
	if err != nil {
		log.WithField("error", err).Error("Failed unmarshalling message")
		return
	}
	log.WithField("type", message.Type).Debug("Received message")

	var replyBytes []byte
	switch message.Type {
	case MsgTypeRegister:
		typedMessage := Register{}
		err = msgpack.Unmarshal(msgBytes, &typedMessage)
		if err != nil {
			log.WithField("error", err).Error("Failed unmarshalling register control message")
			return
		}

		replyBytes, err = agent.handleRegister(&typedMessage)
		if err != nil {
			log.WithField("error", err).Error("Error handling register control message")
			return
		}
	case MsgTypeFetch:
		typedMessage := Fetch{}
		err = msgpack.Unmarshal(msgBytes, &typedMessage)
		if err != nil {
			log.WithField("error", err).Error("Failed unmarshalling fetch control message")
			return
		}

		replyBytes, err = agent.handleFetch(&typedMessage)
		if err != nil {
			log.WithField("error", err).Error("Error handling fetch control message")
			return
		}
	case MsgTypeBundleCreate:
		typedMessage := BundleCreate{}
		err = msgpack.Unmarshal(msgBytes, &typedMessage)
		if err != nil {
			log.WithField("error", err).Error("Failed unmarshalling create control message")
			return
		}

		replyBytes, err = agent.handleCreate(&typedMessage)
		if err != nil {
			log.WithField("error", err).Error("Error handling create control message")
			return
		}

	default:
		log.Debug("Not doing anything with this message")
		return
	}

	replyLength := uint64(len(replyBytes))
	replyLengthBytes := make([]byte, 8)
	_, err = binary.Encode(replyLengthBytes, binary.BigEndian, replyLength)
	if err != nil {
		log.WithField("error", err).Error("Error encoding reply length")
		return
	}

	_, err = connWriter.Write(replyLengthBytes)
	if err != nil {
		log.WithField("error", err).Error("Error sending reply length")
		return
	}
	_, err = connWriter.Write(replyBytes)
	if err != nil {
		log.WithField("error", err).Error("Error sending reply")
		return
	}
	err = connWriter.Flush()
	if err != nil {
		log.WithField("error", err).Error("Error flushing send buffer")
		return
	}
}

func (agent *RECAgent) handleRegister(message *Register) ([]byte, error) {
	log.WithField("eid", message.EndpointID).Debug("Performing registration")
	reply := Reply{
		Message: Message{Type: MsgTypeReply},
		Success: true,
		Error:   "",
	}

	failure := false

	eid, err := bpv7.NewEndpointID(message.EndpointID)
	if err != nil {
		failure = true
		reply.Success = false
		reply.Error = err.Error()
		log.WithFields(log.Fields{
			"eid":   message.EndpointID,
			"error": err,
		}).Debug("Error parsing EndpointID")
	}

	if !failure {
		err = agent.mailboxes.Register(eid)
		if err != nil {
			failure = true
			reply.Success = false
			reply.Error = err.Error()
			log.WithFields(log.Fields{
				"eid":   message.EndpointID,
				"error": err,
			}).Debug("Error performing registration")
		}
	}

	log.Debug("Marshalling reply")
	replyBytes, err := msgpack.Marshal(&reply)
	if err != nil {
		log.WithField("error", err).Error("Reply marshalling error")
		return nil, err
	}

	return replyBytes, nil
}

func (agent *RECAgent) handleFetch(message *Fetch) ([]byte, error) {
	log.WithFields(log.Fields{
		"eid":       message.EndpointID,
		"node type": message.NodeType,
	}).Debug("Handling bundle fetch")
	reply := FetchReply{
		Reply: Reply{
			Message: Message{Type: MsgTypeFetchReply},
			Success: true,
			Error:   "",
		},
	}
	allBundles := make([]*bpv7.Bundle, 0)
	failure := false

	var address bpv7.EndpointID
	var mailbox *application_agent.Mailbox
	var bundles []*bpv7.Bundle

	// get unicast bundles
	address, err := bpv7.NewEndpointID(message.EndpointID)
	if err != nil {
		failure = true
		reply.Success = false
		reply.Error = err.Error()
	}

	if !failure {
		mailbox, err = agent.mailboxes.GetMailbox(address)
		if err != nil {
			failure = true
			reply.Success = false
			reply.Error = err.Error()
		} else {
			bundles, err = mailbox.GetAll(true)
			if err != nil {
				failure = true
				reply.Success = false
				reply.Error = err.Error()
			} else {
				log.WithField("bundles", bundles).Debug("Unicast bundles")
				allBundles = slices.Concat(allBundles, bundles)
			}
		}
	}

	// get multicast bundles
	address = agent.multicastAddresses[message.NodeType]
	if !failure {
		mailbox, err = agent.mailboxes.GetMailbox(address)
		if err != nil {
			failure = true
			reply.Success = false
			reply.Error = err.Error()
		} else {
			bundles, err = mailbox.GetAll(true)
			if err != nil {
				failure = true
				reply.Success = false
				reply.Error = err.Error()
			} else {
				log.WithField("bundles", bundles).Debug("Multicast bundles")
				allBundles = slices.Concat(allBundles, bundles)
			}
		}
	}
	log.WithField("bundles", allBundles).Debug("All bundles")

	allBundleData := make([]BundleData, 0, len(allBundles))
	if !failure {
		for _, bundle := range allBundles {
			var bundleData BundleData
			err := msgpack.Unmarshal(bundle.PayloadBlock.Value.(*bpv7.PayloadBlock).Data(), &bundleData)
			if err == nil {
				allBundleData = append(allBundleData, bundleData)
			} else {
				log.WithFields(log.Fields{
					"bundle": bundle.ID(),
					"error":  err,
				}).Error("Error unmarshalling bundle payload")
			}
		}
	}
	reply.Bundles = allBundleData
	log.WithField("reply", reply).Debug("Fetch reply")

	log.Debug("Marshalling reply")
	replyBytes, err := msgpack.Marshal(&reply)
	if err != nil {
		log.WithField("error", err).Error("Reply marshalling error")
		return nil, err
	}

	return replyBytes, nil
}

func (agent *RECAgent) handleCreate(message *BundleCreate) ([]byte, error) {
	log.Debug("Creating bundle")
	reply := Reply{
		Message: Message{Type: MsgTypeReply},
		Success: true,
		Error:   "",
	}
	failure := false

	srcAddress, err := bpv7.NewEndpointID(message.Bundle.Source)
	if err != nil {
		failure = true
		reply.Success = false
		reply.Error = err.Error()
	}

	var dstAddress bpv7.EndpointID
	if !failure {
		dstAddress, err = bpv7.NewEndpointID(message.Bundle.Destination)
		if err != nil {
			failure = true
			reply.Success = false
			reply.Error = err.Error()
		}
	}

	var payload []byte
	if !failure {
		payload, err = msgpack.Marshal(message.Bundle)
		if err != nil {
			failure = true
			reply.Success = false
			reply.Error = err.Error()
		}
	}

	if !failure {
		typeBlock := bpv7.NewRECBundleTypeBlock(uint8(message.Bundle.Type))
		bldr := bpv7.Builder().Source(srcAddress).Destination(dstAddress).CreationTimestampNow().Lifetime("1h").PayloadBlock(payload).Canonical(typeBlock)

		bndl, err := bldr.Build()
		if err != nil {
			failure = true
			reply.Success = false
			reply.Error = err.Error()
		}

		if !failure {
			application_agent.GetManagerSingleton().Send(bndl)
		}
	}

	log.Debug("Marshalling response")
	replyBytes, err := msgpack.Marshal(&reply)
	if err != nil {
		log.WithField("error", err).Error("Response marshalling error")
		return nil, err
	}

	return replyBytes, nil
}
