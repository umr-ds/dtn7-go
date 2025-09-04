package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"
	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/application_agent"
	"github.com/dtn7/dtn7-go/pkg/application_agent/rec_agent"
	"github.com/dtn7/dtn7-go/pkg/application_agent/rest_agent"
	"github.com/dtn7/dtn7-go/pkg/application_agent/unix_agent"
	"github.com/dtn7/dtn7-go/pkg/cla"
	"github.com/dtn7/dtn7-go/pkg/cla/dummy_cla"
	"github.com/dtn7/dtn7-go/pkg/cla/mtcp"
	"github.com/dtn7/dtn7-go/pkg/cla/quicl"
	"github.com/dtn7/dtn7-go/pkg/discovery"
	"github.com/dtn7/dtn7-go/pkg/id_keeper"
	"github.com/dtn7/dtn7-go/pkg/processing"
	"github.com/dtn7/dtn7-go/pkg/routing"
	"github.com/dtn7/dtn7-go/pkg/store"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s configuration.toml", os.Args[0])
	}

	conf, err := parse(os.Args[1])
	if err != nil {
		log.WithField("error", err).Error("Config error")
		return
	}

	defer stopDtnd()
	err = startDtnd(conf)
	if err != nil {
		log.WithField("errors", err).Error("Errors during daemon startup")
		return
	}

	// wait for SIGINT or SIGTERM
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	sig := <-c
	log.WithField("signal", sig).Info("Received signal, shutting down.")
	return
}

func startDtnd(conf config) (startupErrors error) {
	log.SetLevel(conf.LogLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02T15:04:05.000",
	})

	processing.SetOwnNodeID(conf.NodeID)

	// Setup Store
	err := store.InitialiseStore(conf.NodeID, conf.Store.Path)
	if err != nil {
		err = NewStartupError("Error initialising store", err)
		startupErrors = errors.Join(startupErrors, err)
	}

	// Setup IdKeeper
	id_keeper.InitializeIdKeeper()

	// Setup routing
	var routingAlgorithm routing.Algorithm = nil
	switch conf.Routing.Algorithm {
	case routing.Epidemic:
		routingAlgorithm = routing.NewEpidemicRouting()
	default:
		err = NewStartupError(fmt.Sprintf("%v not valid routing algorithm", conf.Routing.Algorithm), nil)
		startupErrors = errors.Join(startupErrors, err)
	}
	routing.InitialiseAlgorithm(routingAlgorithm)

	// Setup application agents
	err = application_agent.InitialiseApplicationAgentManager(processing.ReceiveBundle)
	if err != nil {
		err = NewStartupError("Error initialising Application Agent Manager", err)
		startupErrors = errors.Join(startupErrors, err)
	}

	if conf.Agents.REST.Address != "" {
		restAgent := rest_agent.NewRestAgent("/rest", conf.Agents.REST.Address)
		err = application_agent.GetManagerSingleton().RegisterAgent(restAgent)
		if err != nil {
			err = NewStartupError("Error registering REST application agent", err)
			startupErrors = errors.Join(startupErrors, err)
		}
	}

	if conf.Agents.UNIX.Socket != "" {
		unixAgent, err := unix_agent.NewUNIXAgent(conf.Agents.UNIX.Socket)
		if err != nil {
			err = NewStartupError("Error creating UNIX application agent", err)
			startupErrors = errors.Join(startupErrors, err)
		}
		err = application_agent.GetManagerSingleton().RegisterAgent(unixAgent)
		if err != nil {
			err = NewStartupError("Error registering UNIX application agent", err)
			startupErrors = errors.Join(startupErrors, err)
		}
	}

	if conf.Agents.REC.Socket != "" {
		recAgent, err := rec_agent.NewRECAgent(conf.Agents.REC.Socket)
		if err != nil {
			log.WithError(err).Fatal("Error creating REC application agent")
		}

		err = application_agent.GetManagerSingleton().RegisterAgent(recAgent)
		if err != nil {
			log.WithError(err).Fatal("Error registering REC application agent")
		}
	}

	// Setup CLAs
	cla.InitialiseCLAManager(processing.ReceiveBundle, processing.NewPeer, routing.GetAlgorithmSingleton().NotifyPeerDisappeared)

	for _, lstConf := range conf.Listener {
		var listener cla.ConvergenceListener
		switch lstConf.Type {
		case cla.Dummy:
			listener = dummy_cla.NewDummyListener(lstConf.Address)
		case cla.MTCP:
			srv := mtcp.NewMTCPServer(lstConf.Address, lstConf.EndpointId, cla.GetManagerSingleton().NotifyReceive)
			listener = srv
			cla.GetManagerSingleton().Register(srv)
		case cla.QUICL:
			listener = quicl.NewQUICListener(lstConf.Address, lstConf.EndpointId, cla.GetManagerSingleton().NotifyReceive)
		default:
			err = NewStartupError(fmt.Sprintf("%v not valid convergence listener type", lstConf.Type), nil)
			startupErrors = errors.Join(startupErrors, err)
			continue
		}

		err = cla.GetManagerSingleton().RegisterListener(listener)
		if err != nil {
			err = NewStartupError("Error starting convergence listener", err)
			startupErrors = errors.Join(startupErrors, err)
		}
	}

	// Setup neighbour discovery
	err = discovery.InitialiseManager(conf.NodeID, conf.Discovery, 2*time.Second, true, false, cla.GetManagerSingleton().NotifyReceive)
	if err != nil {
		err = NewStartupError("Error starting discovery manager", err)
		startupErrors = errors.Join(startupErrors, err)
	}

	s, err := gocron.NewScheduler()
	if err != nil {
		err = NewStartupError("Error initializing cron scheduler", err)
		startupErrors = errors.Join(startupErrors, err)
		return
	}
	_, err = s.NewJob(
		gocron.DurationJob(
			conf.Cron.Dispatch,
		),
		gocron.NewTask(
			processing.DispatchPending,
		),
	)
	if err != nil {
		err = NewStartupError("Error initializing dispatching cronjob", err)
		startupErrors = errors.Join(startupErrors, err)
	}
	_, err = s.NewJob(
		gocron.DurationJob(
			conf.Cron.GC,
		),
		gocron.NewTask(
			gc,
		),
	)
	if err != nil {
		err = NewStartupError("Error initializing garbage collection cronjob", err)
		startupErrors = errors.Join(startupErrors, err)
	}

	s.Start()
	return
}

func stopDtnd() {
	discovery.GetManagerSingleton().Shutdown()
	cla.GetManagerSingleton().Shutdown()
	application_agent.GetManagerSingleton().Shutdown()
	err := store.GetStoreSingleton().Shutdown()
	if err != nil {
		log.WithField("error", err).Error("Error shutting down store")
	}
}

func gc() {
	store.GetStoreSingleton().GarbageCollect()
	application_agent.GetManagerSingleton().GC()
}

type StartupError struct {
	msg   string
	cause error
}

func NewStartupError(msg string, cause error) *StartupError {
	err := StartupError{
		msg:   msg,
		cause: cause,
	}
	return &err
}

func (err *StartupError) Error() string {
	return err.msg
}

func (err *StartupError) Unwrap() error {
	return err.cause
}
