package pullcid

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	ipfspb "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/ipfs/pb"
)

// client
type IpfsClientProtocol struct {
	customProtocol.CustomStreamClientProtocol
}

var pullCidClientProtocol *IpfsClientProtocol

func GetClientProtocol() (*IpfsClientProtocol, error) {
	if pullCidClientProtocol == nil {
		pullCidClientProtocol = &IpfsClientProtocol{}
		err := pullCidClientProtocol.Init()
		if err != nil {
			return nil, err
		}
	}
	return pullCidClientProtocol, nil
}

func (p *IpfsClientProtocol) Init() error {
	p.CustomStreamClientProtocol.Init(pid, customProtocol.DataType_PROTO3)
	return nil
}

func (p *IpfsClientProtocol) Request(ctx context.Context, peerId string, ipfsReq *ipfspb.IpfsReq) (chan *ipfspb.IpfsRes, error) {

	log.Debugf("IpfsClientProtocol->Request begin:\npeerId: %s \nrequest: %v", peerId, ipfsReq)
	_, err := cid.Decode(ipfsReq.CID)
	if err != nil {
		log.Errorf("IpfsClientProtocol->Request: cid.Decode: err: %v, cid: %s", err, ipfsReq.CID)
		return nil, err
	}

	_, customProtocolRespChan, err := p.CustomStreamClientProtocol.Request(peerId, ipfsReq)
	if err != nil {
		log.Errorf("IpfsClientProtocol->Request: err: %v", err)
		return nil, err
	}

	ret := make(chan *ipfspb.IpfsRes)
	go func() {
		select {
		case data := <-customProtocolRespChan:
			customProtocolResponse, ok := data.(*pb.CustomProtocolRes)
			if !ok {
				log.Errorf("IpfsClientProtocol->Request: responseChan is not CustomProtocolRes")
				ret <- nil
				return
			}
			response := &ipfspb.IpfsRes{}
			err := p.CustomStreamClientProtocol.HandleResponse(customProtocolResponse, response)
			if err != nil {
				log.Errorf("IpfsClientProtocol->Request: CustomStreamClientProtocol.HandleResponse error: %v", err)
				ret <- nil
				return
			}
			ret <- response
			return
		case <-p.Ctx.Done():
			ret <- nil
			return
		case <-ctx.Done():
			ret <- nil
			return
		}
	}()
	return ret, nil
}
