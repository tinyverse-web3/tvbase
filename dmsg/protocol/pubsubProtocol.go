package protocol

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MailboxPProtocol struct {
	PubsubProtocol
	Callback MailboxPpCallback
}

type QueryPeerProtocol struct {
	PubsubProtocol
	Callback QueryPeerCallback
}

type PubsubMsgProtocol struct {
	PubsubProtocol
	Callback PubsubMsgCallback
}

type PubsubProtocol struct {
	Protocol
}

func (p *PubsubProtocol) HandleRequestData(requestProtocolData []byte, dataList ...any) error {
	log.Logger.Debugf("PubsubProtocol->HandleRequestData begin:\ndataList: %v", dataList)

	request, response, abort, err := p.Protocol.HandleRequestData(requestProtocolData)
	if abort {
		return nil
	}
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

	// send the response
	responseBasicData, err := GetBasicData(response)
	if err != nil {
		log.Logger.Errorf("PubsubProtocol->HandleRequestData: GetResponseBasicData error: %+v", err)
		return err
	}

	requestBasicData, err := GetBasicData(response)
	if err != nil {
		log.Logger.Errorf("PubsubProtocol->HandleRequestData: GetRequestBasicData error: %+v", err)
		return err
	}

	target, err := p.Service.GetPublishTarget(requestBasicData.Pubkey)
	if err != nil {
		return err
	}
	err = p.Service.PublishProtocol(p.Ctx, target, responseBasicData.PID, responseProtoData)
	if err != nil {
		log.Logger.Errorf("PubsubProtocol->HandleRequestData: PublishProtocol error: %v", err)
	}
	log.Logger.Debugf("PubsubProtocol->HandleRequestData end")
	return nil
}

func (p *PubsubProtocol) Request(reqPubkey string, destPubkey string, dataList ...any) (protoreflect.ProtoMessage, chan any, error) {
	log.Logger.Debugf("PubsubProtocol->Request begin:\nreqPubkey: %s", reqPubkey)

	dataList = append([]any{destPubkey}, dataList...)
	requestInfoId, request, requestProtoData, err := p.GenRequestInfo(reqPubkey, dataList...)
	if err != nil {
		return nil, nil, err
	}

	requestBasicData, err := GetBasicData(request)
	if err != nil {
		log.Logger.Errorf("PubsubProtocol->Request: GetRequestBasicData error: %+v", err)
		return nil, nil, err
	}

	target, err := p.Service.GetPublishTarget(destPubkey)
	if err != nil {
		return nil, nil, err
	}
	err = p.Service.PublishProtocol(p.Ctx, target, requestBasicData.PID, requestProtoData)
	if err != nil {
		log.Logger.Errorf("PubsubProtocol->Request: PublishProtocol error: %v", err)
		p.RequestInfoList.Delete(requestInfoId)
		return nil, nil, err
	}

	log.Logger.Debugf("PubsubProtocol->Request end")
	requestInfoData, _ := p.RequestInfoList.Load(requestBasicData.ID)
	return request, requestInfoData.(*RequestInfo).ResponseChan, nil
}

func NewQueryPeerProtocol(
	ctx context.Context,
	host host.Host,
	callback QueryPeerCallback,
	dmsg DmsgService,
	adapter PpAdapter) *QueryPeerProtocol {
	ret := &QueryPeerProtocol{}
	ret.Ctx = ctx
	ret.Host = host
	ret.Callback = callback
	ret.Service = dmsg
	ret.Adapter = adapter
	go ret.TickCleanRequest()
	return ret
}

func NewPubsubMsgProtocol(
	ctx context.Context,
	host host.Host,
	callback PubsubMsgCallback,
	dmsg DmsgService,
	adapter PpAdapter) *PubsubMsgProtocol {
	ret := &PubsubMsgProtocol{}
	ret.Ctx = ctx
	ret.Host = host
	ret.Callback = callback
	ret.Service = dmsg
	ret.Adapter = adapter
	go ret.TickCleanRequest()
	return ret
}

func NewMailboxPProtocol(
	ctx context.Context,
	host host.Host,
	callback MailboxPpCallback,
	dmsg DmsgService,
	adapter PpAdapter) *MailboxPProtocol {
	ret := &MailboxPProtocol{}
	ret.Ctx = ctx
	ret.Host = host
	ret.Callback = callback
	ret.Service = dmsg
	ret.Adapter = adapter
	go ret.TickCleanRequest()
	return ret
}
