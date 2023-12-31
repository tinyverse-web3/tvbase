package customProtocol

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgCommonKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"

	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/pubsub"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/stream"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
	dmsgProtocolCustom "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
)

type CustomProtocolClient struct {
	CustomProtocolBase
	queryPeerProtocol        *basic.QueryPeerProtocol
	clientStreamProtocolList map[string]*dmsgProtocolCustom.ClientStreamProtocol
}

func NewClient(tvbaseService define.TvBaseService, pubkey string, getSig dmsgCommonKey.GetSigCallback) (*CustomProtocolClient, error) {
	d := &CustomProtocolClient{}
	err := d.Init(tvbaseService, pubkey, getSig)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// sdk-common
func (d *CustomProtocolClient) Init(tvbaseService define.TvBaseService, pubkey string, getSig dmsgCommonKey.GetSigCallback) error {
	log.Debug("CustomProtocolClient->Start begin")
	err := d.LightUserService.Init(tvbaseService)
	if err != nil {
		return err
	}

	err = d.LightUserService.Start(pubkey, getSig, false)
	if err != nil {
		return err
	}

	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()

	if d.queryPeerProtocol == nil {
		d.queryPeerProtocol = pubsub.NewQueryPeerProtocol(ctx, host, d, d)
		d.RegistPubsubProtocol(d.queryPeerProtocol.Adapter.GetResponsePID(), d.queryPeerProtocol)
	}

	if d.clientStreamProtocolList == nil {
		d.clientStreamProtocolList = make(map[string]*dmsgProtocolCustom.ClientStreamProtocol)
	}

	log.Debug("CustomProtocolClient->Start end")
	return nil
}

func (d *CustomProtocolClient) Release() error {
	log.Debug("CustomProtocolClient->Release begin")
	d.UnregistPubsubProtocol(d.queryPeerProtocol.Adapter.GetResponsePID())
	d.queryPeerProtocol = nil

	err := d.LightUserService.Stop()
	if err != nil {
		return err
	}
	d.clientStreamProtocolList = nil
	log.Debug("CustomProtocolClient->Release end")
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
		log.Errorf("CustomProtocolClient->Request: err: %v, servicePeerInfo: %v, user public key: %s",
			err, peerID, d.LightUser.Key.PubkeyHex)
		return nil, nil, err
	}
	return request.(*pb.CustomProtocolReq), responseChan, nil
}

func (d *CustomProtocolClient) RegistClient(client dmsgProtocolCustom.ClientHandle, pubkey string) error {
	customProtocolID := client.GetProtocolID()
	if d.clientStreamProtocolList[customProtocolID] != nil {
		log.Errorf("CustomProtocolClient->RegistCSPClient: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("CustomProtocolClient->RegistCSPClient: protocol %s is already exist", customProtocolID)
	}
	d.clientStreamProtocolList[customProtocolID] = &dmsgProtocolCustom.ClientStreamProtocol{
		Protocol: stream.NewCustomStreamProtocol(d.TvBase.GetCtx(), d.TvBase.GetHost(), customProtocolID, d, d, false, pubkey),
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
