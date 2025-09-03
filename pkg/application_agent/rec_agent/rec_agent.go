package rec_agent

import (
	"net"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/application_agent"
	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/store"
)

const RECBrokerMulticastAddress = "dtn://rec.broker/~"

type RECAgent struct {
	listenAddress *net.UnixAddr
	listener      *net.UnixListener
	mailboxes     *application_agent.MailboxBank
	stopChan      chan interface{}
}

func NewRECAgent(listenAddress string) (*RECAgent, error) {
	unixAddr, err := net.ResolveUnixAddr("unix", listenAddress)
	if err != nil {
		return nil, err
	}

	brokerAddr := bpv7.MustNewEndpointID(RECBrokerMulticastAddress)

	agent := RECAgent{
		listenAddress: unixAddr,
		mailboxes:     application_agent.NewMailboxBank(),
		stopChan:      make(chan interface{}),
	}

	_ = agent.mailboxes.Register(brokerAddr)

	return &agent, nil
}

func (agent *RECAgent) Shutdown() {
	log.WithField("listenAddress", agent.listenAddress).Info("Shutting RECAgent down")
	close(agent.stopChan)
	f, _ := agent.listener.File()
	_ = f.Close()
	_ = agent.listener.Close()
}

func (agent *RECAgent) Endpoints() []bpv7.EndpointID {
	return agent.mailboxes.RegisteredIDs()
}

func (agent *RECAgent) Deliver(bundleDescriptor *store.BundleDescriptor) error {
	return agent.mailboxes.Deliver(bundleDescriptor)
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

}
