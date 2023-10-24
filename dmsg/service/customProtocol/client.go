package customProtocol

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgCommonKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	dmsgProtocolCustom "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
)

type CustomProtocolClient struct {
	CustomProtocolBase
	queryPeerProtocol        *dmsgProtocol.QueryPeerProtocol
	clientStreamProtocolList map[string]*dmsgProtocolCustom.ClientStreamProtocol
}

func CreateClient(tvbaseService define.TvBaseService) (*CustomProtocolClient, error) {
	d := &CustomProtocolClient{}
	err := d.Init(tvbaseService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *CustomProtocolClient) Init(tvbaseService define.TvBaseService) error {
	err := d.LightUserService.Init(tvbaseService)
	if err != nil {
		return err
	}
	d.clientStreamProtocolList = make(map[string]*dmsgProtocolCustom.ClientStreamProtocol)
	return nil
}

// sdk-common
func (d *CustomProtocolClient) Start(pubkey string, getSig dmsgCommonKey.GetSigCallback) error {
	log.Debug("CustomProtocolClient->Start begin")
	err := d.LightUserService.Start(pubkey, getSig, false)
	if err != nil {
		return err
	}

	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()

	d.queryPeerProtocol = adapter.NewQueryPeerProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(d.queryPeerProtocol.Adapter.GetResponsePID(), d.queryPeerProtocol)

	log.Debug("CustomProtocolClient->Start end")
	return nil
}

func (d *CustomProtocolClient) Stop() error {
	log.Debug("CustomProtocolClient->Stop begin")
	err := d.LightUserService.Stop()
	if err != nil {
		return err
	}
	log.Debug("CustomProtocolClient->Stop end")
	return nil
}

func (d *CustomProtocolClient) QueryPeer(pid string) (*pb.QueryPeerReq, chan any, error) {
	destPubkey := d.GetQueryPeerPubkey()
	request, responseChan, err := d.queryPeerProtocol.Request(d.LightUser.Key.PubkeyHex, destPubkey, pid)
	return request.(*pb.QueryPeerReq), responseChan, err
}

func (d *CustomProtocolClient) Request(peerIdStr string, pid string, content []byte) (*pb.CustomProtocolReq, chan any, error) {
	protocolInfo := d.clientStreamProtocolList[pid]
	if protocolInfo == nil {
		log.Errorf("CustomProtocolClient->Request: protocol %s is not exist", pid)
		return nil, nil, fmt.Errorf("CustomProtocolClient->Request: protocol %s is not exist", pid)
	}

	peerID, err := peer.Decode(peerIdStr)
	if err != nil {
		log.Errorf("CustomProtocolClient->Request: err: %v", err)
		return nil, nil, err
	}
	request, responseChan, err := protocolInfo.Protocol.Request(
		peerID,
		d.LightUser.Key.PubkeyHex,
		pid,
		content)
	if err != nil {
		log.Errorf("CustomProtocolClient->Request: err: %v, servicePeerInfo: %v, user public key: %s, content: %v",
			err, peerID, d.LightUser.Key.PubkeyHex, content)
		return nil, nil, err
	}
	return request.(*pb.CustomProtocolReq), responseChan, nil
}

func (d *CustomProtocolClient) RegistClient(client dmsgProtocolCustom.ClientHandle) error {
	customProtocolID := client.GetProtocolID()
	if d.clientStreamProtocolList[customProtocolID] != nil {
		log.Errorf("CustomProtocolClient->RegistCSPClient: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("CustomProtocolClient->RegistCSPClient: protocol %s is already exist", customProtocolID)
	}
	d.clientStreamProtocolList[customProtocolID] = &dmsgProtocolCustom.ClientStreamProtocol{
		Protocol: adapter.NewCustomStreamProtocol(d.TvBase.GetCtx(), d.TvBase.GetHost(), customProtocolID, d, d, false),
		Handle:   client,
	}
	client.SetCtx(d.TvBase.GetCtx())
	client.SetService(d)
	return nil
}

func (d *CustomProtocolClient) UnregistClient(client dmsgProtocolCustom.ClientHandle) error {
	customProtocolID := client.GetProtocolID()
	if d.clientStreamProtocolList[customProtocolID] == nil {
		log.Warnf("CustomProtocolClient->UnregistCSPClient: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.clientStreamProtocolList[customProtocolID] = nil
	return nil
}
