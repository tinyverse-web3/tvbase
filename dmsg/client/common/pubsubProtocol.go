package common

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (p *PubsubProtocol) HandleRequestData(requestProtocolData []byte) error {
	dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData begin\nrequestPID: %v", p.Adapter.GetRequestPID())

	requestProtoMsg, responseProtoMsg, err := p.Protocol.HandleRequestData(requestProtocolData)
	if err != nil {
		if requestProtoMsg == nil {
			return err
		}
		responseProtoMsg, err = p.GetErrResponse(requestProtoMsg, err)
		if err != nil {
			return err
		}
	}
	if requestProtoMsg == nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: requestProtoMsg is nil")
		return err
	}

	if responseProtoMsg == nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: responseProtoMsg is nil")
		return err
	}

	responseProtoData, err := proto.Marshal(responseProtoMsg)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: marshal response error: %v", err)
		return err
	}
	// send the response
	requestBasicData := p.Adapter.GetRequestBasicData(requestProtoMsg)
	responseBasicData := p.Adapter.GetResponseBasicData(responseProtoMsg)
	err = p.Service.PublishProtocol(p.Ctx, requestBasicData.Pubkey, responseBasicData.PID, responseProtoData)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: PublishProtocol error: %v", err)
	}
	dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData end")
	return nil
}

func (p *PubsubProtocol) Request(
	srcUserPubKey string,
	destUserPubkey string,
	proxyPubkey string,
	dataList ...any) (protoreflect.ProtoMessage, chan any, error) {
	dmsgLog.Logger.Debugf("PubsubProtocol->Request begin:\nsrcUserPubKey:%s", srcUserPubKey)

	dataList = append([]any{destUserPubkey}, dataList...)
	requestInfoId, requestProtoMsg, requestProtoData, err := p.GenRequestInfo(srcUserPubKey, proxyPubkey, dataList...)
	if err != nil {
		return nil, nil, err
	}

	requestBasicData := p.Adapter.GetRequestBasicData(requestProtoMsg)
	err = p.Service.PublishProtocol(p.Ctx, destUserPubkey, requestBasicData.PID, requestProtoData)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request: PublishProtocol error: %v", err)
		p.RequestInfoList.Delete(requestInfoId)
		return nil, nil, err
	}

	dmsgLog.Logger.Debugf("PubsubProtocol->Request end")
	requestInfoData, _ := p.RequestInfoList.Load(requestBasicData.ID)
	return requestProtoMsg, requestInfoData.(*RequestInfo).DoneChan, nil
}

func NewPubsubProtocol(
	ctx context.Context,
	host host.Host,
	protocolCallback PubsubProtocolCallback,
	protocolService ProtocolService,
	adapter PubsubProtocolAdapter) *PubsubProtocol {
	ret := &PubsubProtocol{}
	ret.Ctx = ctx
	ret.Host = host
	ret.Callback = protocolCallback
	ret.Service = protocolService
	ret.Adapter = adapter
	go ret.TickCleanRequest()
	return ret
}
