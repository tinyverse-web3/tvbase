package tvbase

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"sync"
	"syscall"
	"time"

	logging "github.com/ipfs/go-log"
	"github.com/ipfs/go-metrics-interface"
	"github.com/libp2p/go-libp2p"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	libp2pEvent "github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	ma "github.com/multiformats/go-multiaddr"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/db"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	tvPeer "github.com/tinyverse-web3/tvbase/common/peer"
	tvProtocol "github.com/tinyverse-web3/tvbase/common/protocol"
	tvutil "github.com/tinyverse-web3/tvbase/common/util"
	dkvs "github.com/tinyverse-web3/tvbase/dkvs"
	dmsgClient "github.com/tinyverse-web3/tvbase/dmsg/client"
	dmsgService "github.com/tinyverse-web3/tvbase/dmsg/service"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/dig"
	"go.uber.org/fx"
)

type TvBase struct {
	DmsgService                 tvCommon.DmsgService
	DkvsService                 tvCommon.DkvsService
	ctx                         context.Context
	host                        host.Host
	dht                         *kaddht.IpfsDHT
	dhtDatastore                db.Datastore
	nodeCfg                     *config.NodeConfig
	lightPeerListMutex          sync.Mutex
	servicePeerListMutex        sync.Mutex
	servicePeerList             tvPeer.PeerInfoList
	lightPeerList               tvPeer.PeerInfoList
	connectedCbList             []tvPeer.ConnectCallback
	notConnectedCbList          []tvPeer.ConnectCallback
	nodeInfoService             *tvProtocol.NodeInfoService
	mdnsService                 mdns.Service
	pubRoutingDiscovery         *drouting.RoutingDiscovery
	evtPeerConnectednessChanged libp2pEvent.Subscription
	resourceManager             network.ResourceManager
	relayPeerSignal             chan peer.AddrInfo
	TracerSpan                  trace.Span
	closeSync                   sync.Once
	app                         *fx.App
}

// new tvbase
func NewTvbase(options ...any) (*TvBase, error) {
	var isStart bool = true
	var rootPath string = ""
	ctx := context.Background()
	var ok bool = false

	if len(options) > 0 {
		rootPath, ok = options[0].(string)
		if !ok {
			tvLog.Logger.Errorf("NewInfrasture: options[0](rootPath) is not string")
			return nil, fmt.Errorf("NewInfrasture: options[0](rootPath) is not string")
		}
	}

	if len(options) > 1 {
		ctx, ok = options[1].(context.Context)
		if !ok {
			tvLog.Logger.Errorf("NewInfrasture: options[1](ctx) is not context.Context")
			return nil, fmt.Errorf("NewInfrasture: options[1](ctx) is not context.Context")
		}
	}

	if len(options) > 2 {
		isStart, ok = options[2].(bool)
		if !ok {
			tvLog.Logger.Errorf("NewInfrasture: options[0](isStart) is not bool")
			return nil, fmt.Errorf("NewInfrasture: options[0](isStart) is not bool")
		}
	}

	m := &TvBase{
		ctx: ctx,
	}

	err := m.init(rootPath)
	if err != nil {
		return m, err
	}
	tvLog.Logger.Infof("tvnode mode: %v", m.nodeCfg.Mode)

	if isStart {
		err = m.Start()
		if err != nil {
			return m, err
		}
	}
	return m, nil
}

func (m *TvBase) Start() error {
	switch m.nodeCfg.Mode {
	case config.LightMode:
	case config.FullMode:
		m.initMetric()
	}

	err := m.initEvent()
	if err != nil {
		return err
	}

	err = m.initPeer()
	if err != nil {
		return err
	}

	err = m.initProtocol()
	if err != nil {
		return err
	}

	err = m.bootstrap()
	if err != nil {
		tvLog.Logger.Warn(err)
		return nil
	}

	switch m.nodeCfg.Mode {
	case config.LightMode:
		err := m.initMdns()
		if err != nil {
			return err
		}
	case config.FullMode:
	}

	err = m.initRendezvous()
	if err != nil {
		return err
	}
	go m.DiscoverRendezvousPeers()

	err = m.DmsgService.Start()
	if err != nil {
		return err
	}

	m.netCheck()

	return nil
}

func (m *TvBase) Stop() {
	m.closeSync.Do(func() {
		if m.DmsgService != nil {
			err := m.DmsgService.Stop()
			if err != nil {
				tvLog.Logger.Error(err)
			}
			m.DmsgService = nil
		}
		if err := m.disableMdns(); err != nil {
			tvLog.Logger.Error(err)
		}
		if m.resourceManager != nil {
			m.resourceManager.Close()
			m.resourceManager = nil
		}

		if m.host != nil {
			err := m.host.Close()
			if err != nil {
				tvLog.Logger.Error(err)
			}
			m.host = nil
		}
		if m.dht != nil {
			m.dht.Close()
			m.dht = nil
		}
		if m.app != nil {
			stopErr := m.app.Stop(context.Background())
			if stopErr != nil {
				tvLog.Logger.Errorf("Infrasture->Stop: failure on stop: %v", stopErr)
			}
			m.app = nil
		}
	})
}

func (m *TvBase) initDisc() (fx.Option, error) {
	var fxOpts []fx.Option

	// trace init
	ctx := logging.ContextWithLoggable(m.ctx, newUUID("session"))
	shutdownTracerProvider, err := NewTracerProvider(ctx)
	if err != nil {
		tvLog.Logger.Errorf("Infrasture->initDisc: NewTracerProvider error: %v", err)
		return nil, err
	}

	otel.SetTracerProvider(shutdownTracerProvider)
	otel.SetTextMapPropagator(autoprop.NewTextMapPropagator())
	tracer := shutdownTracerProvider.Tracer("tinverseInfrasture")
	initTraceOpt := fx.Provide(func(lc fx.Lifecycle) trace.Tracer {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				err := shutdownTracerProvider.Shutdown(ctx)
				if err != nil {
					tvLog.Logger.Errorf("Infrasture->initDisc->OnStop: Shutdown error: %v", err)
				}
				return nil
			},
		})
		return tracer
	})
	fxOpts = append(fxOpts, initTraceOpt)

	// profile
	if m.nodeCfg.Disc.EnableProfile {
		var profileOpt fx.Option
		stopProfilingFunc, err := profileIfEnabled()
		if err != nil {
			tvLog.Logger.Errorf("Infrasture->initDisc: %v", err)
			return nil, err
		}

		profileOpt = fx.Provide(func(lc fx.Lifecycle) func() {
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					stopProfilingFunc()
					return nil
				},
			})
			return stopProfilingFunc
		})
		fxOpts = append(fxOpts, profileOpt)
	}

	var intrOpt fx.Option
	switch m.nodeCfg.Mode {
	case config.LightMode:
	case config.FullMode:
		// interrupt
		intrh := NewIntrHandler()
		var cancelFunc context.CancelFunc
		ctx, cancelFunc = context.WithCancel(ctx)
		handlerFunc := func(count int, ih *IntrHandler) {
			switch count {
			case 1:
				fmt.Println() // Prevent un-terminated ^C character in terminal

				ih.wg.Add(1)
				go func() {
					defer ih.wg.Done()
					cancelFunc()
					m.Stop()
				}()

			default:
				fmt.Println("Received another interrupt before graceful shutdown, terminating...")
				os.Exit(-1)
			}
		}
		intrh.Handle(handlerFunc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

		intrOpt = fx.Provide(func(lc fx.Lifecycle) *IntrHandler {
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					intrh.Close()
					return nil
				},
			})
			return intrh
		})
		fxOpts = append(fxOpts, intrOpt)
	}

	// trace start
	var traceArguments []string // TODO add arguments
	traceOpt := trace.WithAttributes(attribute.StringSlice("Arguments", traceArguments))
	spanName := "tinverseInfrasture"
	ctx, m.TracerSpan = tracer.Start(
		ctx,
		spanName,
		traceOpt,
	)

	startTraceOpt := fx.Provide(func(lc fx.Lifecycle) trace.Span {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				m.TracerSpan.End()
				return nil
			},
		})
		return m.TracerSpan
	})
	fxOpts = append(fxOpts, startTraceOpt)

	// metrics
	m.ctx = metrics.CtxScope(ctx, "tvInfrasture")
	return fx.Options(fxOpts...), nil
}

func (m *TvBase) initFx(opt fx.Option) error {
	fxOpts := []fx.Option{
		fx.NopLogger,
		opt,
	}
	m.app = fx.New(fxOpts...)

	if m.app.Err() != nil {
		return logAndUnwrapFxError(m.app.Err())
	}

	if err := m.app.Start(m.ctx); err != nil {
		return logAndUnwrapFxError(err)
	}
	return nil
}

func (m *TvBase) init(rootPath string) error {
	var err error
	fullPath, err := tvutil.GetRootPath(rootPath)
	if err != nil {
		tvLog.Logger.Errorf("tvbase->init: error: %v", err)
		return err
	}

	err = m.initConfig(fullPath)
	if err != nil {
		tvLog.Logger.Errorf("tvbase->init: error: %v", err)
		return err
	}

	privateKey, swamPsk, err := m.initKey(fullPath)
	if err != nil {
		tvLog.Logger.Errorf("tvbase->init: error: %v", err)
		return err
	}

	m.nodeCfg.RootPath = fullPath

	rand.Seed(time.Now().UnixNano())

	// disc
	fxOpt, err := m.initDisc()
	if err != nil {
		tvLog.Logger.Errorf("tvbase->init: error: %v", err)
		return err
	}

	// fx
	err = m.initFx(fxOpt)
	if err != nil {
		tvLog.Logger.Errorf("tvbase->init: error: %v", err)
		return err
	}

	// libp2p node
	var nodeOpts []libp2p.Option
	nodeOpts, err = m.createOpts(m.ctx, privateKey, swamPsk)
	if err != nil {
		tvLog.Logger.Errorf("tvbase->init: error: %v", err)
		return err
	}

	m.host, err = libp2p.New(nodeOpts...)
	if err != nil {
		tvLog.Logger.Errorf("tvbase->init: error: %v", err)
		return err
	}

	tvLog.Logger.Debugf("hostID:%s,Addresses:", m.host.ID())
	for _, addr := range m.host.Addrs() {
		tvLog.Logger.Debugf("\t%s/p2p/%s", addr, m.host.ID())
	}

	m.DkvsService = dkvs.NewDkvs(m)

	switch m.nodeCfg.Mode {
	case config.LightMode:
		m.DmsgService, err = dmsgClient.CreateService(m)
	case config.FullMode:
		m.DmsgService, err = dmsgService.CreateService(m)
	}
	if err != nil {
		tvLog.Logger.Errorf("infrasture->init: error: %v", err)
		return err
	}
	return nil
}

func (m *TvBase) bootstrap() error {
	// Bootstrap the persistence DHT. In the default configuration, this spawns a Background, thread that will refresh the peer table every five minutes.
	tvLog.Logger.Info("Bootstrapping the persistence DHT")
	if err := m.dht.Bootstrap(m.ctx); err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, peerInfo := range m.nodeCfg.Bootstrap.BootstrapPeers {
		maddr, err := ma.NewMultiaddr(peerInfo)
		if err != nil {
			tvLog.Logger.Errorf("infrasture->bootstrap: fail to parse bootstrap peer:%v, error:%v", peerInfo, err)
			return err
		}
		addrInfo, _ := peer.AddrInfoFromP2pAddr(maddr)
		if addrInfo.ID == m.host.ID() {
			continue
		}
		m.host.Network().(*swarm.Swarm).Backoff().Clear(addrInfo.ID) //Prevent repeated connection failures
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := m.host.Connect(m.ctx, *addrInfo)
			if err != nil {
				tvLog.Logger.Warnf("Infrasture->bootstrap: fail connect boottrap addrInfo:%v, error:%v", addrInfo, err)
			} else {
				m.RegistServicePeer(addrInfo.ID)
				tvLog.Logger.Infof("Infrasture->bootstrap: succ connect bootstrap node:%v", addrInfo)
			}
		}()
	}
	wg.Wait()
	return nil
}

func (m *TvBase) netCheck() {
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()

		<-t.C
		for {
			select {
			case <-t.C:
				if len(m.host.Network().Peers()) == 0 {
					tvLog.Logger.Warn("Infrasture->netCheck: We are in private network and have no peers, might be configuration mistake, try to connect bootstrap peer node again")
					err := m.bootstrap()
					if err != nil {
						tvLog.Logger.Warnf("tinverseInfrasture-netCheck: fail to connect bootstrap peer node, error: %v", err)
					}
				}
			case <-m.ctx.Done():
				return
			}
		}
	}()
}

func (m *TvBase) GetClientDmsgService() *dmsgClient.DmsgService {
	return m.DmsgService.(*dmsgClient.DmsgService)
}

func (m *TvBase) GetServiceDmsgService() *dmsgService.DmsgService {
	return m.DmsgService.(*dmsgService.DmsgService)
}

func (m *TvBase) GetDkvsService() tvCommon.DkvsService {
	return m.DkvsService
}

func (m *TvBase) GetDhtDatabase() db.Datastore {
	return m.dhtDatastore
}

func (m *TvBase) GetNodeConnectedness() network.Connectedness {
	return m.host.Network().Connectedness(m.host.ID())
}

// Log the entire `app.Err()` but return only the innermost one to the user
// given the full error can be very long (as it can expose the entire build
// graph in a single string).
//
// The fx.App error exposed through `app.Err()` normally contains un-exported
// errors from its low-level `dig` package:
// * https://github.com/uber-go/dig/blob/5e5a20d/error.go#L82
// These usually wrap themselves in many layers to expose where in the build
// chain did the error happen. Although useful for a developer that needs to
// debug it, it can be very confusing for a user that just wants the IPFS error
// that he can probably fix without being aware of the entire chain.
// Unwrapping everything is not the best solution as there can be useful
// information in the intermediate errors, mainly in the next to last error
// that locates which component is the build error coming from, but it's the
// best we can do at the moment given all errors in dig are private and we
// just have the generic `RootCause` API.
func logAndUnwrapFxError(fxAppErr error) error {
	if fxAppErr == nil {
		return nil
	}

	tvLog.Logger.Error("constructing the node: ", fxAppErr)

	err := fxAppErr
	for {
		extractedErr := dig.RootCause(err)
		// Note that the `RootCause` name is misleading as it just unwraps only
		// *one* error layer at a time, so we need to continuously call it.
		if !reflect.TypeOf(extractedErr).Comparable() {
			// Some internal errors are not comparable (e.g., `dig.errMissingTypes`
			// which is a slice) and we can't go further.
			break
		}
		if extractedErr == err {
			// We didn't unwrap any new error in the last call, reached the innermost one.
			break
		}
		err = extractedErr
	}

	return fmt.Errorf("constructing the node (see log for full detail): %w", err)
}
