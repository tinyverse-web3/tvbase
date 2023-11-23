package basic

import (
	"fmt"

	"github.com/tinyverse-web3/tvbase/dmsg/protocol/common"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/log"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MailboxPProtocol struct {
	PubsubProtocol
	Callback common.MailboxPpCallback
}

type QueryPeerProtocol struct {
	PubsubProtocol
	Callback common.QueryPeerCallback
}

type PubsubMsgProtocol struct {
	PubsubProtocol
	Callback common.PubsubMsgCallback
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
	responseBasicData, err := util.GetBasicData(response)
	if err != nil {
		log.Logger.Errorf("PubsubProtocol->HandleRequestData: GetResponseBasicData error: %+v", err)
		return err
	}

	requestBasicData, err := util.GetBasicData(request)
	if err != nil {
		log.Logger.Errorf("PubsubProtocol->HandleRequestData: GetRequestBasicData error: %+v", err)
		return err
	}

	pubkey := requestBasicData.Pubkey
	if requestBasicData.ProxyPubkey != "" {
		pubkey = requestBasicData.ProxyPubkey
	}

	target, err := p.Service.GetPublishTarget(pubkey)
	if err != nil {
		return err
	}
	log.Logger.Debugf("PubsubProtocol->HandleRequestData response : target is %v", target)
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

	requestBasicData, err := util.GetBasicData(request)
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
