package tvbase

// import (
// 	libp2pEvent "github.com/libp2p/go-libp2p/core/event"
// 	"github.com/libp2p/go-libp2p/core/network"
// 	tvCommon "github.com/tinyverse-web3/tvbase/common"
// 	tvLog "github.com/tinyverse-web3/tvbase/common/log"
// )

// func (m *TvBase) registNetReachabilityChanged(n tvCommon.NoArgCallback) error {
// 	// 订阅EEvtPeerConnectednessChanged事件
// 	h := m.GetHost()
// 	reachabilityChanged, err := h.EventBus().Subscribe(&libp2pEvent.EvtPeerConnectednessChanged{})
// 	if err != nil {
// 		tvLog.Logger.Error("failed to subscribe to event EvtPeerConnectednessChanged, err=%s", err)
// 		return err
// 	}

// 	// 启动事件监听协程
// 	go func() {
// 		for {
// 			select {
// 			case v := <-reachabilityChanged.Out():
// 				var evt libp2pEvent.EvtPeerConnectednessChanged
// 				evt, ok := v.(libp2pEvent.EvtPeerConnectednessChanged)
// 				if !ok {
// 					tvLog.Logger.Errorf("unexpected event:%v", v)
// 					continue
// 				}
// 				tvLog.Logger.Debugf("Infrasture->RegistNetReachabilityChanged: peer connectedness changed-> connectedness:%v, peer:%v",
// 					evt.Connectedness, evt.Peer)
// 				switch evt.Connectedness {
// 				case network.NotConnected:
// 					tvLog.Logger.Debug("Infrasture->RegistNetReachabilityChanged: network is not connected")
// 				case network.Connected:
// 					tvLog.Logger.Debug("Infrasture->RegistNetReachabilityChanged: network is connected")
// 					tvLog.Logger.Debug("Infrasture->RegistNetReachabilityChanged: ---Start sync all local key to other peers---")
// 					n()
// 					tvLog.Logger.Debug("Infrasture->RegistNetReachabilityChanged: ---Complete sync all local key to other peers---")
// 				case network.CanConnect:
// 					// never happen
// 					tvLog.Logger.Debug("Infrasture->RegistNetReachabilityChanged: never happen CanConnect")
// 				case network.CannotConnect:
// 					// never happen
// 					tvLog.Logger.Debug("Infrasture->RegistNetReachabilityChanged: never happen CannotConnect")
// 				}
// 				continue
// 			case <-m.GetCtx().Done():
// 				if reachabilityChanged != nil {
// 					err := reachabilityChanged.Close()
// 					if err != nil {
// 						tvLog.Logger.Errorf("Infrasture->RegistNetReachabilityChanged: evtPeerConnectednessChanged.Close() error: %v", err)
// 					}
// 					reachabilityChanged = nil
// 				}
// 				return
// 			}
// 		}
// 	}()
// 	return nil
// }

// func (m *TvBase) ConnectBootstrapNode() {
// 	m.bootstrap()
// }

// func (m *TvBase) GetDhtProtocolPrefix() string {
// 	return m.nodeCfg.DHT.DatastorePath
// }
