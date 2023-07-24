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

	err := p.Protocol.HandleRequestData(requestProtocolData)
	if err != nil {
		return err
	}

	responseProtoData, err := proto.Marshal(p.ResponseProtoMsg)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: marshal response error: %v", err)
		return err
	}
	// send the response
	requestBasicData := p.Adapter.GetRequestBasicData()
	responseBasicData := p.Adapter.GetResponseBasicData()
	err = p.Service.PublishProtocol(requestBasicData.Pubkey, responseBasicData.PID, responseProtoData)

	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: PublishProtocol error:%v", err)
	}

	dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData end")
	return nil
}

func (p *PubsubProtocol) Request(
	srcUserPubKey string,
	destUserPubkey string,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	dmsgLog.Logger.Debugf("PubsubProtocol->Request begin:\nsrcUserPubKey:%s", srcUserPubKey)

	dataList = append([]any{destUserPubkey}, dataList...)
	requestInfoId, _, requestProtoData, err := p.GenRequestInfo(srcUserPubKey, dataList...)
	if err != nil {
		return nil, err
	}

	requestBasicData := p.Adapter.GetRequestBasicData()
	err = p.Service.PublishProtocol(destUserPubkey, requestBasicData.PID, requestProtoData)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request: PublishProtocol error: %v", err)
		delete(p.RequestInfoList, requestInfoId)
		return nil, err
	}

	dmsgLog.Logger.Debugf("PubsubProtocol->Request end")
	return p.RequestProtoMsg, nil
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
	ret.RequestInfoList = make(map[string]*RequestInfo)
	ret.Adapter = adapter
	go ret.TickCleanRequest()
	return ret
}
