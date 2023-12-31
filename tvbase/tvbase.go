package tvbase

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"

	logging "github.com/ipfs/go-log"
	"github.com/ipfs/go-metrics-interface"
	"github.com/libp2p/go-libp2p"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoremem"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/db"
	"github.com/tinyverse-web3/tvbase/common/define"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	tvPeer "github.com/tinyverse-web3/tvbase/common/peer"
	tvProtocol "github.com/tinyverse-web3/tvbase/common/protocol"
	dkvs "github.com/tinyverse-web3/tvbase/dkvs"
	dmsgPubsub "github.com/tinyverse-web3/tvbase/dmsg/common/pubsub"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/dig"
	"go.uber.org/fx"
)

type TvBase struct {
	DkvsService            define.DkvsService
	TracerSpan             trace.Span
	ctx                    context.Context
	host                   host.Host
	dht                    *kaddht.IpfsDHT
	dhtDatastore           db.Datastore
	cfg                    *config.TvbaseConfig
	lightPeerListMutex     sync.Mutex
	servicePeerListMutex   sync.Mutex
	servicePeerList        tvPeer.PeerInfoList
	lightPeerList          tvPeer.PeerInfoList
	connectedCbList        []tvPeer.ConnectCallback
	notConnectedCbList     []tvPeer.ConnectCallback
	nodeInfoService        *tvProtocol.NodeInfoService
	pubRoutingDiscovery    *drouting.RoutingDiscovery
	isRendezvous           bool
	isDiscoverRendzvousing bool
	rendezvousChanList     []chan bool
	launch                 *fx.App
	rootPath               string
}

// new tvbase
func NewTvbase(ctx context.Context, cfg *config.TvbaseConfig, rootPath string) (*TvBase, error) {
	m := &TvBase{
		ctx:      ctx,
		cfg:      cfg,
		rootPath: rootPath,
	}

	err := m.init()
	if err != nil {
		return m, err
	}
	switch m.cfg.Mode {
	case config.LightMode:
		tvLog.Logger.Infof("NewTvbase: mode: %s", "LightMode")
	case config.ServiceMode:
		tvLog.Logger.Infof("NewTvbase: mode: %s", "ServiceMode")
	default:
		tvLog.Logger.Errorf("NewTvbase: mode is not exist: mode: %s", m.cfg.Mode)
		return m, fmt.Errorf("NewTvbase: mode is not exist: mode: %v", m.cfg.Mode)
	}
	return m, nil
}

func (m *TvBase) Start() error {
	switch m.cfg.Mode {
	case config.LightMode:
	case config.ServiceMode:
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
	}

	err = m.initRendezvous()
	if err != nil {
		return err
	}
	go m.DiscoverRendezvousPeers()

	if err := m.launch.Start(m.ctx); err != nil {
		return logAndUnwrapFxError(err)
	}
	return nil
}

func (m *TvBase) Stop() error {
	return m.launch.Stop(context.Background())
}

func (m *TvBase) initDisc() (fx.Option, error) {
	var fxOpts []fx.Option

	// trace init
	ctx := logging.ContextWithLoggable(m.ctx, newUUID("session"))
	shutdownTracerProvider, err := NewTracerProvider(ctx)
	if err != nil {
		tvLog.Logger.Errorf("tvBase->initDisc: NewTracerProvider error: %v", err)
		return nil, err
	}

	otel.SetTracerProvider(shutdownTracerProvider)
	otel.SetTextMapPropagator(autoprop.NewTextMapPropagator())
	tracer := shutdownTracerProvider.Tracer("tvbase")
	initTraceOpt := fx.Provide(func(lc fx.Lifecycle) trace.Tracer {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				err := shutdownTracerProvider.Shutdown(ctx)
				if err != nil {
					tvLog.Logger.Errorf("tvBase->initDisc->OnStop: Shutdown error: %v", err)
				}
				return nil
			},
		})
		return tracer
	})
	fxOpts = append(fxOpts, initTraceOpt)

	// profile
	if m.cfg.Disc.EnableProfile {
		var profileOpt fx.Option
		stopProfilingFunc, err := profileIfEnabled()
		if err != nil {
			tvLog.Logger.Errorf("tvBase->initDisc: %v", err)
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
	switch m.cfg.Mode {
	case config.LightMode:
	case config.ServiceMode:
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
	spanName := "tvbase"
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
	m.ctx = metrics.CtxScope(ctx, "tvbase")
	return fx.Options(fxOpts...), nil
}

func (m *TvBase) initFx(opt fx.Option) error {
	fxOpts := []fx.Option{
		fx.NopLogger,
		opt,
	}
	m.launch = fx.New(fxOpts...)

	if m.launch.Err() != nil {
		return logAndUnwrapFxError(m.launch.Err())
	}

	return nil
}

func (m *TvBase) checkListenAddrs() error {
	for _, addr := range m.cfg.Network.ListenAddrs {
		subkeys := strings.Split(addr, "/")
		l := len(subkeys)
		if l < 5 {
			continue
		}

		addrStr := ""
		switch subkeys[1] {
		case "ip4":
			addrStr = subkeys[2] + ":" + subkeys[4]
		case "ip6":
			addrStr = "[" + subkeys[2] + "]" + ":" + subkeys[4]
		}
		network := subkeys[3]
		switch network {
		case "tcp":
			conn, err := net.Listen(network, addrStr)
			if err != nil {
				tvLog.Logger.Errorf("TvBase->checkListenAddrs: Listen, addr: %v, error: %v", subkeys, err)
				return err
			}
			conn.Close()
		case "udp":
			addr, err := net.ResolveUDPAddr(network, addrStr)
			if err != nil {
				tvLog.Logger.Errorf("TvBase->checkListenAddrs: ResolveUDPAddr, addr: %v, error: %v", addrStr, err)
				return err
			}
			conn, err := net.ListenUDP(network, addr)
			if err != nil {
				tvLog.Logger.Errorf("TvBase->checkListenAddrs: ListenUDP, addr: %v, error: %v", addrStr, err)
				return err
			}
			conn.Close()
		}

	}
	return nil

}

func (m *TvBase) initHost(lc fx.Lifecycle, privateKey crypto.PrivKey, swamPsk pnet.PSK) (host.Host, error) {
	err := m.checkListenAddrs()
	if err != nil {
		return nil, err
	}

	filepath := m.GetRootPath() + m.cfg.DHT.DatastorePath
	m.dhtDatastore, err = db.CreateDataStore(filepath, m.cfg.Mode)
	if err != nil {
		tvLog.Logger.Errorf("tvbase->createOpts->createDataStore: error: %v", err)
		return nil, err
	}

	// common
	var nodeOpts []libp2p.Option
	nodeOpts, err = m.createCommonOpts(privateKey, swamPsk)
	if err != nil {
		tvLog.Logger.Errorf("tvbase->initHost->createCommonOpts: error: %v", err)
		return nil, err
	}

	// route
	routeOpt, err := m.createRouteOpt()
	if err != nil {
		tvLog.Logger.Errorf("tvbase->createOpts->createRouteOpt: error: %v", err)
		return nil, err
	}
	nodeOpts = append(nodeOpts, routeOpt)
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return m.dht.Close()
		},
	})

	// resource manager
	switch m.cfg.Mode {
	case config.ServiceMode:
		rmgr, err := m.initResourceManager()
		if err != nil {
			tvLog.Logger.Errorf("tvbase->createCommonOpts: error: %v", err)
			return nil, err
		}
		nodeOpts = append(nodeOpts, libp2p.ResourceManager(rmgr))
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return rmgr.Close()
			},
		})
	}

	// peerstore
	pstore, err := pstoremem.NewPeerstore()
	if err != nil {
		return nil, err
	}
	// add self peer
	publicKey := privateKey.GetPublic()
	peerId, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	if err := pstore.AddPubKey(peerId, publicKey); err != nil {
		return nil, err
	}
	if err := pstore.AddPrivKey(peerId, privateKey); err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return pstore.Close()
		},
	})
	nodeOpts = append(nodeOpts, libp2p.Peerstore(pstore))

	m.host, err = libp2p.New(nodeOpts...)
	if err != nil {
		tvLog.Logger.Errorf("tvbase->initHost: error: %v", err)
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return m.host.Close()
		},
	})

	tvLog.Logger.Infof("tvbase->initHost: hostID: %s", m.host.ID())

	listenType := ""
	addrs, err := m.host.Network().InterfaceListenAddresses()
	if err != nil {
		listenType = "host.Addrs"
		addrs = m.host.Addrs()
		tvLog.Logger.Errorf("tvbase->initHost: InterfaceListenAddresses error: %v", err)
	} else {
		listenType = "host.Network().InterfaceListenAddresses"
	}

	addrListInfo := ""
	for _, addr := range addrs {
		addrListInfo += fmt.Sprintf("addr: %v/p2p/%s\n", addr, m.host.ID())
	}
	tvLog.Logger.Infof("tvbase->initHost: %s:\n%s", listenType, addrListInfo)

	return m.host, nil
}

func (m *TvBase) init() error {
	rand.Seed(time.Now().UnixNano())

	// init fx launch
	var fxOpts []fx.Option
	fxOpt, err := m.initDisc()
	if err != nil {
		tvLog.Logger.Errorf("tvbase->init: error: %v", err)
		return err
	}
	fxOpts = append(fxOpts, fxOpt)
	fxOpts = append(fxOpts, fx.Provide(m.initKey))
	fxOpts = append(fxOpts, fx.Provide(m.initHost))
	fxOpts = append(fxOpts, fx.Invoke(m.initMdns))
	fxOpts = append(fxOpts, fx.Invoke(m.netCheck))
	fxOpts = append(fxOpts, fx.Invoke(m.initDmsgPubsub))
	err = m.initFx(fx.Options(fxOpts...))
	if err != nil {
		tvLog.Logger.Errorf("tvbase->init: error: %v", err)
		return err
	}

	// dkvs service
	dkvsServer := dkvs.NewDkvs(m)
	if dkvsServer == nil {
		dkvsErr := fmt.Errorf("failed to create dkvs service")
		tvLog.Logger.Errorf("tvbase->init: error: %v", dkvsErr)
		return dkvsErr
	}
	m.DkvsService = dkvsServer
	return nil
}

func (m *TvBase) initDmsgPubsub(lc fx.Lifecycle) error {
	cfg := m.cfg.DMsg.Pubsub
	var optList []pubsub.Option
	if cfg != nil && cfg.TraceFile != "" {
		tracer, err := pubsub.NewJSONTracer(m.rootPath + cfg.TraceFile)
		if err != nil {
			return err
		}
		opt := pubsub.WithEventTracer(tracer)
		optList = append(optList, opt)
	}
	dmsgPubsub.NewPubsubMgr(m.ctx, m.host, optList...)
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})
	return nil
}

func (m *TvBase) bootstrap() error {
	// Bootstrap the persistence DHT. In the default configuration, this spawns a Background, thread that will refresh the peer table every five minutes.
	tvLog.Logger.Debug("tvBase->bootstrap begin")
	if err := m.dht.Bootstrap(m.ctx); err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, bootstrapPeer := range m.cfg.Bootstrap.BootstrapPeers {
		tvLog.Logger.Debugf("tvBase->bootstrap:\nbootstrapPeer: %+v", bootstrapPeer)
		mulitAddr, err := ma.NewMultiaddr(bootstrapPeer)
		if err != nil {
			tvLog.Logger.Errorf("tvBase->bootstrap: ma.NewMultiaddr error: %+v", err)
			return err
		}
		tvLog.Logger.Debugf("tvBase->bootstrap:\nmulitAddr: %+v", mulitAddr)
		peerAddrInfo, err := peer.AddrInfoFromP2pAddr(mulitAddr)
		if err != nil {
			tvLog.Logger.Errorf("tvBase->bootstrap: peer.AddrInfoFromP2pAddr error: %+v", err)
			return err
		}
		tvLog.Logger.Debugf("tvBase->bootstrap:\npeerAddrInfo: %+v", peerAddrInfo)
		if peerAddrInfo.ID == m.host.ID() {
			continue
		}

		// m.host.Network().(*swarm.Swarm).Backoff().Clear(peerAddrInfo.ID) //Prevent repeated connection failures
		wg.Add(1)
		go func(addrInfo *peer.AddrInfo) {
			defer wg.Done()
			err := m.host.Connect(m.ctx, *addrInfo)
			if err != nil {
				tvLog.Logger.Warnf("tvBase->bootstrap: host.Connect error: %+v", err)
			} else {
				m.RegistServicePeer(addrInfo.ID)
				tvLog.Logger.Infof("tvBase->bootstrap: succ connect \naddrInfo: %+v", addrInfo)
			}
		}(peerAddrInfo)
	}
	wg.Wait()
	m.PrintDiagnosisInfo()
	tvLog.Logger.Debug("tvBase->bootstrap end")
	return nil
}

func (m *TvBase) netCheck(ph host.Host, lc fx.Lifecycle) error {
	done := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				t := time.NewTicker(30 * time.Second)
				defer t.Stop()

				<-t.C // swallow one tick
				for {
					select {
					case <-t.C:
						if !m.IsExistConnectedPeer() {
							tvLog.Logger.Warn("tvBase->netCheck: no peers")
							err := m.bootstrap()
							if err == nil {
								if !m.isDiscoverRendzvousing {
									m.isRendezvous = false
									go m.DiscoverRendezvousPeers()
								}
							}
						}
					case <-done:
						return
					case <-m.ctx.Done():
						return
					}
				}
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			close(done)
			return nil
		},
	})
	return nil
}

func (m *TvBase) GetDkvsService() define.DkvsService {
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

func (m *TvBase) PrintDiagnosisInfo() *define.DiagnosisInfo {
	ret := &define.DiagnosisInfo{
		Host:                   m.host,
		Dht:                    m.dht,
		IsRendezvous:           m.isRendezvous,
		IsDiscoverRendzvousing: m.isDiscoverRendzvousing,
		ServicePeerList:        m.servicePeerList,
		LightPeerList:          m.lightPeerList,
		NetworkPeers:           m.host.Network().Peers(),
	}
	outPrint := ""
	outPrint += "TvBase->PrintDiagnosisInfo\n"
	mode := ""
	switch m.cfg.Mode {
	case config.LightMode:
		mode = "LightMode"
	case config.ServiceMode:
		mode = "ServiceMode"
	}
	outPrint += fmt.Sprintf("hostID: %s	mode: %s\nisDiscoverRendzvousing: %v	isRendezvous: %v\n",
		m.host.ID().String(),
		mode,
		m.isDiscoverRendzvousing,
		m.isRendezvous,
	)
	outPrint += "ServicePeerList:\n"
	for _, peer := range m.servicePeerList {
		outPrint += fmt.Sprintf("	ID: %s, status: %v\n", peer.PeerID.String(), peer.ConnectStatus)
	}
	outPrint += "LightPeerList:\n"
	for _, peer := range m.lightPeerList {
		outPrint += fmt.Sprintf("	ID: %s, status: %v\n", peer.PeerID.String(), peer.ConnectStatus)
	}
	peerstore := m.host.Peerstore()
	outPrint += "host.Network.Peers:\n"
	for _, peer := range m.host.Network().Peers() {
		peerInfo := peerstore.PeerInfo(peer)
		outPrint += fmt.Sprintf("	peerID: %s\n", peerInfo.ID.String())
		for _, addr := range peerInfo.Addrs {
			outPrint += fmt.Sprintf("		addr: %s\n", addr.String())
		}
	}
	outPrint = strings.TrimSuffix(outPrint, "\n")
	tvLog.Logger.Info(outPrint)
	return ret
}
