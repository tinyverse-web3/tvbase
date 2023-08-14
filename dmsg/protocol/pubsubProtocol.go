package protocol

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MailboxPProtocol struct {
	PubsubProtocol
	Callback MailboxPpCallback
}

type PubsubMsgProtocol struct {
	PubsubProtocol
	Callback PubsubMsgCallback
}

type PubsubProtocol struct {
	Protocol
}

func (p *PubsubProtocol) HandleRequestData(
	requestProtocolData []byte,
	dataList ...any) error {
	log.Logger.Debugf(
		"PubsubProtocol->HandleRequestData begin\nrequestProtocolData: %v,\ndataList: %v",
		requestProtocolData, dataList)

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
	requestBasicData := p.Adapter.GetRequestBasicData(request)
	responseBasicData := p.Adapter.GetResponseBasicData(response)
	target, err := p.Service.GetPublishTarget(requestBasicData.Pubkey)
	if err != nil {
		return err
	}
	err = p.Service.PublishProtocol(target, responseBasicData.PID, responseProtoData)
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
	target, err := p.Service.GetPublishTarget(destUserPubkey)
	if err != nil {
		return nil, nil, err
	}
	err = p.Service.PublishProtocol(target, requestBasicData.PID, requestProtoData)
	if err != nil {
		log.Logger.Errorf("PubsubProtocol->Request: PublishProtocol error: %v", err)
		delete(p.RequestInfoList, requestInfoId)
		return nil, nil, err
	}

	log.Logger.Debugf("PubsubProtocol->Request end")
	return requestProtoMsg, p.RequestInfoList[requestBasicData.ID].DoneChan, nil
}

func NewPubsubMsgProtocol(
	ctx context.Context,
	host host.Host,
	callback PubsubMsgCallback,
	dmsg DmsgServiceInterface,
	adapter PpAdapter) *PubsubMsgProtocol {
	ret := &PubsubMsgProtocol{}
	ret.Ctx = ctx
	ret.Host = host
	ret.Callback = callback
	ret.Service = dmsg
	ret.RequestInfoList = make(map[string]*RequestInfo)
	ret.Adapter = adapter
	go ret.TickCleanRequest()
	return ret
}

func NewMailboxPProtocol(
	ctx context.Context,
	host host.Host,
	callback MailboxPpCallback,
	dmsg DmsgServiceInterface,
	adapter PpAdapter) *MailboxPProtocol {
	ret := &MailboxPProtocol{}
	ret.Ctx = ctx
	ret.Host = host
	ret.Callback = callback
	ret.Service = dmsg
	ret.RequestInfoList = make(map[string]*RequestInfo)
	ret.Adapter = adapter
	go ret.TickCleanRequest()
	return ret
}
