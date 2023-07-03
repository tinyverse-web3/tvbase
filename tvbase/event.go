package tvbase

import (
	libp2pEvent "github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/network"
	libp2pPeer "github.com/libp2p/go-libp2p/core/peer"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	tvPeer "github.com/tinyverse-web3/tvbase/common/peer"
)

func (m *TvBase) initEvent() error {
	m.connectedCbList = make([]tvPeer.ConnectCallback, 0)
	m.notConnectedCbList = make([]tvPeer.ConnectCallback, 0)

	var err error
	if m.evtPeerConnectednessChanged == nil {
		m.evtPeerConnectednessChanged, err = m.host.EventBus().Subscribe(&libp2pEvent.EvtPeerConnectednessChanged{})
	}
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case v := <-m.evtPeerConnectednessChanged.Out():
				var evt libp2pEvent.EvtPeerConnectednessChanged
				evt, ok := v.(libp2pEvent.EvtPeerConnectednessChanged)
				if !ok {
					tvLog.Logger.Errorf("unexpected event:%v", v)
					continue
				}
				tvLog.Logger.Debugf("Infrasture->initEvent: peer connectedness changed-> connectedness:%v, peer:%v",
					evt.Connectedness, evt.Peer)
				switch evt.Connectedness {
				case network.NotConnected:
					// tvLog.Logger.Debug("NotConnected")
					m.NotifyNotConnected(evt.Peer)
				case network.Connected:
					// tvLog.Logger.Debug("Connected")
					m.NotifyConnected(evt.Peer)
				case network.CanConnect:
					// never happen
					tvLog.Logger.Debug("Infrasture->initEvent: never happen CanConnect")
				case network.CannotConnect:
					// never happen
					tvLog.Logger.Debug("Infrasture->initEvent: never happen CannotConnect")
				}
				continue
			case <-m.ctx.Done():
				if m.evtPeerConnectednessChanged != nil {
					err := m.evtPeerConnectednessChanged.Close()
					if err != nil {
						tvLog.Logger.Errorf("Infrasture->initEvent: evtPeerConnectednessChanged.Close() error: %v", err)
					}
					m.evtPeerConnectednessChanged = nil
				}
				return
			}
		}
	}()
	return nil
}

func (m *TvBase) RegistConnectedCallback(callback tvPeer.ConnectCallback) {
	m.connectedCbList = append(m.connectedCbList, callback)
}

func (m *TvBase) RegistNotConnectedCallback(callback tvPeer.ConnectCallback) {
	m.notConnectedCbList = append(m.notConnectedCbList, callback)
}

func (m *TvBase) NotifyConnected(peerID libp2pPeer.ID) {
	m.TrySetPeerStatus(peerID, network.Connected)
	for _, callback := range m.connectedCbList {
		err := callback(peerID)
		if err != nil {
			tvLog.Logger.Errorf("Infrasture->NotifyConnected: callback error: %v", err)
		}
	}
}

func (m *TvBase) NotifyNotConnected(peerID libp2pPeer.ID) {
	m.TrySetPeerStatus(peerID, network.NotConnected)
	for _, callback := range m.notConnectedCbList {
		err := callback(peerID)
		if err != nil {
			tvLog.Logger.Errorf("Infrasture->NotifyNotConnected: callback error: %v", err)
		}
	}
}

func (m *TvBase) TrySetPeerStatus(peerID libp2pPeer.ID, status network.Connectedness) {
	peerId := peerID.String()
	if m.servicePeerList[peerId] != nil {
		m.servicePeerList[peerId].ConnectStatus = status
	} else if m.lightPeerList[peerId] != nil {
		m.lightPeerList[peerId].ConnectStatus = status
	}
}