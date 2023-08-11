package protocol

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type PubsubProtocol struct {
	Protocol
	Callback PubsubProtocolCallback
}

func (p *PubsubProtocol) HandleRequestData(
	requestProtocolData []byte,
	dataList ...any) error {
	log.Logger.Debugf(
		"PubsubProtocol->HandleRequestData begin\nrequestProtocolData: %v,\ndataList: %v",
		requestProtocolData, dataList)

	request, response, err := p.Protocol.HandleRequestData(requestProtocolData)
	if err != nil {
		if request == nil {
			return err
		}
		response, err = p.GetErrResponse(request, err)
		if err != nil {
			return err
		}
	}
	if request == nil {
		log.Logger.Errorf("PubsubProtocol->HandleRequestData: request is nil")
		return fmt.Errorf("PubsubProtocol->HandleRequestData: request is nil")
	}
	if response == nil {
		log.Logger.Errorf("PubsubProtocol->HandleRequestData: response is nil")
		return fmt.Errorf("PubsubProtocol->HandleRequestData: response is nil")
	}

	responseProtoData, err := proto.Marshal(response)
	if err != nil {
		log.Logger.Errorf("PubsubProtocol->HandleRequestData: marshal response error: %v", err)
		return err
	}
	requestBasicData := p.Adapter.GetRequestBasicData(request)
	responseBasicData := p.Adapter.GetResponseBasicData(response)

	// send the response
	err = p.Service.PublishProtocol(requestBasicData.Pubkey, responseBasicData.PID, responseProtoData)
	if err != nil {
		log.Logger.Errorf("PubsubProtocol->HandleRequestData: PublishProtocol error: %v", err)
	}
	log.Logger.Debugf("PubsubProtocol->HandleRequestData end")
	return nil
}

func (p *PubsubProtocol) Request(
	srcUserPubKey string,
	destUserPubkey string,
	dataList ...any) (protoreflect.ProtoMessage, chan any, error) {
	log.Logger.Debugf(
		"PubsubProtocol->Request begin:\nsrcUserPubKey: %s\ndestUserPubkey: %s\ndataList: %v",
		srcUserPubKey, destUserPubkey, dataList)

	dataList = append([]any{destUserPubkey}, dataList...)
	requestInfoId, requestProtoMsg, requestProtoData, err := p.GenRequestInfo(srcUserPubKey, dataList...)
	if err != nil {
		return nil, nil, err
	}

	requestBasicData := p.Adapter.GetRequestBasicData(requestProtoMsg)
	err = p.Service.PublishProtocol(destUserPubkey, requestBasicData.PID, requestProtoData)
	if err != nil {
		log.Logger.Errorf("PubsubProtocol->Request: PublishProtocol error: %v", err)
		delete(p.RequestInfoList, requestInfoId)
		return nil, nil, err
	}

	log.Logger.Debugf("PubsubProtocol->Request end")
	return requestProtoMsg, p.RequestInfoList[requestBasicData.ID].DoneChan, nil
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
