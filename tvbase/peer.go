package tvbase

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/network"
	libp2pPeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/tinyverse-web3/tvbase/common/config"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	tvPeer "github.com/tinyverse-web3/tvbase/common/peer"
	tvPb "github.com/tinyverse-web3/tvbase/common/protocol/pb"
)

func (m *TvBase) initPeer() error {
	m.servicePeerList = make(tvPeer.PeerInfoList)
	m.lightPeerList = make(tvPeer.PeerInfoList)
	return nil
}

func (m *TvBase) registPeerInfo(peerID libp2pPeer.ID) {
	id := peerID.String()
	if m.lightPeerList[id] != nil {
		return
	}
	if m.servicePeerList[id] != nil {
		return
	}
	result := m.nodeInfoService.Request(m.ctx, peerID)
	if result == nil {
		tvLog.Logger.Errorf("tvBase->registPeerInfo: try get peer info: %v, result is nil", peerID)
		return
	}
	if result.Error != nil {
		tvLog.Logger.Errorf("tvBase->registPeerInfo: try get peer info: %v happen error, result: %v", peerID, result)
		return
	}

	if result.NodeInfo == nil {
		tvLog.Logger.Warnf("tvBase->registPeerInfo: result node info is nil: pereID %v", peerID)
		return
	}
	switch result.NodeInfo.NodeType {
	case tvPb.NodeType_Light:
		m.RegistLightPeer(peerID)
	case tvPb.NodeType_Full:
		m.RegistServicePeer(peerID)
	}
}

func (m *TvBase) RegistServicePeer(peerID libp2pPeer.ID) error {
	m.servicePeerListMutex.Lock()
	defer m.servicePeerListMutex.Unlock()
	m.servicePeerList[peerID.String()] = &tvPeer.PeerInfo{
		PeerID:        peerID,
		ConnectStatus: network.Connected,
	}
	return nil
}

func (m *TvBase) RegistLightPeer(peerID libp2pPeer.ID) error {
	m.lightPeerListMutex.Lock()
	defer m.lightPeerListMutex.Unlock()
	m.lightPeerList[peerID.String()] = &tvPeer.PeerInfo{
		PeerID:        peerID,
		ConnectStatus: network.Connected,
	}
	return nil
}

func (m *TvBase) IsExistConnectedPeer() bool {
	for _, peerInfo := range m.lightPeerList {
		if peerInfo.ConnectStatus == network.Connected {
			return true
		}
	}
	for _, peerInfo := range m.servicePeerList {
		if peerInfo.ConnectStatus == network.Connected {
			return true
		}
	}
	return false
}

func (d *TvBase) GetServicePeerList() tvPeer.PeerInfoList {
	return d.servicePeerList
}

func (m *TvBase) GetLightPeerList() tvPeer.PeerInfoList {
	return m.lightPeerList
}

func (m *TvBase) getAvailablePeerList(key string, nodeMode config.NodeMode) ([]libp2pPeer.ID, error) {
	var findedPeerList []libp2pPeer.ID
	closestPeerList, err := m.dht.GetClosestPeers(m.ctx, key)
	if err != nil {
		tvLog.Logger.Errorf("tvBase->getAvailablePeerList: no find peers, err :%v", err)
		return findedPeerList, err
	}

	var peerList tvPeer.PeerInfoList
	switch nodeMode {
	case config.ServiceMode:
		peerList = m.servicePeerList
	case config.LightMode:
		peerList = m.lightPeerList
	default:
		return nil, fmt.Errorf("tvBase->getAvailablePeerList: invalid node mode")
	}

	for _, closestPeer := range closestPeerList {
		closestPeerId := closestPeer.String()
		peerInfo := peerList[closestPeerId]
		if peerInfo == nil {
			tvLog.Logger.Debugf("tvBase->getAvailablePeerList: peer %v is not exist in peerList", closestPeer)
			continue
		}
		if peerInfo.PeerID == m.host.ID() {
			continue
		}
		if peerInfo.ConnectStatus != network.Connected {
			tvLog.Logger.Debugf("tvBase->getAvailablePeerList: peer %v is not connected", closestPeer)
		}
		findedPeerList = append(findedPeerList, closestPeer)
	}
	if len(findedPeerList) == 0 {
		tvLog.Logger.Error("tvBase->getAvailablePeerList: no available peer found")
		return findedPeerList, fmt.Errorf("tvBase->getAvailablePeerList: no available peer found")
	}
	return findedPeerList, nil
}
