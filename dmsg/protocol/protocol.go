package protocol

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type RequestInfo struct {
	ProtoMessage    protoreflect.ProtoMessage
	CreateTimestamp int64
	ResponseChan    chan any
}

type Protocol struct {
	Ctx             context.Context
	Host            host.Host
	RequestInfoList map[string]*RequestInfo
	Service         DmsgServiceInterface
	Adapter         Adapter
}

func (p *Protocol) HandleRequestData(requestProtoData []byte, dataList ...any) (
	protoreflect.ProtoMessage,
	protoreflect.ProtoMessage, bool, error) {
	log.Logger.Debugf("Protocol->HandleRequestData begin\ndataList: %v", dataList)

	defer func() {
		if r := recover(); r != nil {
			log.Logger.Errorf("Protocol->HandleRequestData: recovered from: err: %v", r)
		}
	}()

	request := p.Adapter.GetEmptyRequest()
	err := proto.Unmarshal(requestProtoData, request)
	if err != nil {
		log.Logger.Errorf("Protocol->HandleRequestData: unmarshal request error: %v", err)
		return nil, nil, false, err
	}

	log.Logger.Debugf("Protocol->HandleRequestData:\nrequest: %v", request)

	requestBasicData := p.Adapter.GetRequestBasicData(request)
	valid := AuthProtoMsg(request, requestBasicData)
	if !valid {
		log.Logger.Errorf("Protocol->HandleRequestData: failed to authenticate message")
		return request, nil, false, fmt.Errorf("Protocol->HandleRequestData: failed to authenticate message")
	}

	userPubkeyHex, err := p.Service.GetUserPubkeyHex()
	if err != nil {
		log.Logger.Errorf("Protocol->HandleRequestData: GetUserPubkeyHex error: %+v", err)
		return request, nil, false, err
	}

	requestCallbackData, retCodeData, abort, err := p.Adapter.CallRequestCallback(request)
	if err != nil || abort {
		return request, nil, abort, err
	}

	// generate response message
	responseBasicData := NewBasicData(p.Host, userPubkeyHex, p.Adapter.GetResponsePID())
	responseBasicData.ID = requestBasicData.ID
	response, err := p.Adapter.InitResponse(request, responseBasicData, requestCallbackData, retCodeData)
	if err != nil {
		log.Logger.Errorf("Protocol->HandleRequestData: InitResponse error: %v", err)
		return request, nil, false, err
	}

	// sign the data
	responseProtoData, err := proto.Marshal(response)
	if err != nil {
		log.Logger.Errorf("Protocol->HandleRequestData: marshal response error: %v", err)
		return request, nil, false, err
	}
	sig, err := p.Service.GetUserSig(responseProtoData)
	if err != nil {
		return request, nil, false, err
	}
	err = p.Adapter.SetResponseSig(response, sig)
	if err != nil {
		log.Logger.Errorf("Protocol->HandleRequestData: SetResponseSig error: %v", err)
		return request, nil, false, err
	}
	log.Logger.Debugf("Protocol->HandleRequestData: protocolResponse: %v", response)

	log.Logger.Debugf("Protocol->HandleRequestData end")
	return request, response, false, nil
}

func (p *Protocol) GetErrResponse(
	request protoreflect.ProtoMessage,
	err error) (protoreflect.ProtoMessage, error) {
	responseErr := err
	requestBasicData := p.Adapter.GetRequestBasicData(request)
	userPubkeyHex, err := p.Service.GetUserPubkeyHex()
	if err != nil {
		log.Logger.Errorf("Protocol->GetErrResponse: GetUserPubkeyHex error: %v", err)
		return request, err
	}
	responseBasicData := NewBasicData(
		p.Host,
		userPubkeyHex,
		p.Adapter.GetResponsePID(),
	)
	responseBasicData.ID = requestBasicData.ID
	responseProtoMsg, err := p.Adapter.InitResponse(
		request, responseBasicData, nil, NewFailRetCode(responseErr.Error()))
	if err != nil {
		log.Logger.Warnf("Protocol->GetErrResponse: InitResponse error: %v", err)
	}
	p.Adapter.SetResponseRetCode(responseProtoMsg, -1, responseErr.Error())
	responseProtoData, err := proto.Marshal(responseProtoMsg)
	if err != nil {
		log.Logger.Errorf("Protocol->GetErrResponse: marshal response error: %v", err)
		return responseProtoMsg, err
	}
	sig, err := p.Service.GetUserSig(responseProtoData)
	if err != nil {
		return responseProtoMsg, err
	}
	err = p.Adapter.SetResponseSig(responseProtoMsg, sig)
	if err != nil {
		log.Logger.Errorf("Protocol->GetErrResponse: SetResponseSig error: %v", err)
		return responseProtoMsg, err
	}
	return responseProtoMsg, nil
}

func (p *Protocol) HandleResponseData(
	responseProtoData []byte,
	dataList ...any) error {
	log.Logger.Debugf("Protocol->HandleResponseData begin:\nresponsePID:%s", p.Adapter.GetResponsePID())
	defer func() {
		if r := recover(); r != nil {
			log.Logger.Errorf("Protocol->HandleResponseData: recovered from: r: %v", r)
		}
	}()

	responseProtoMsg := p.Adapter.GetEmptyResponse()
	err := proto.Unmarshal(responseProtoData, responseProtoMsg)
	if err != nil {
		log.Logger.Errorf("Protocol->HandleResponseData: unmarshal responseProtoMsg error: %+v", err)
		return err
	}

	log.Logger.Debugf("Protocol->HandleResponseData:\nResponseProtoMsg: %+v", responseProtoMsg)

	responseBasicData := p.Adapter.GetResponseBasicData(responseProtoMsg)
	valid := AuthProtoMsg(responseProtoMsg, responseBasicData)
	if !valid {
		log.Logger.Errorf("Protocol->HandleResponseData:\nfailed to authenticate message, responseProtoMsg: %+v", responseProtoMsg)
		return fmt.Errorf("Protocol->HandleResponseData: failed to authenticate message, responseProtoMsg: %+v", responseProtoMsg)
	}

	requestInfo, ok := p.RequestInfoList[responseBasicData.ID]
	log.Logger.Debugf("Protocol->HandleResponseData:\nrequestInfo: %+v", requestInfo)
	if ok && requestInfo != nil {
		_, err := p.Adapter.CallResponseCallback(requestInfo.ProtoMessage, responseProtoMsg)
		if err != nil {
			log.Logger.Warnf("Protocol->HandleResponseData:\nCallResponseCallback: error %v", err)
		}
		select {
		case requestInfo.ResponseChan <- responseProtoMsg:
			log.Logger.Debugf("Protocol->HandleResponseData: succ send ResponseChan")
		default:
			log.Logger.Debugf("Protocol->HandleResponseData: no receiver for ResponseChan")
		}
		// delete for mulit pubsub response
		// close(requestInfo.DoneChan)
		// delete(p.RequestInfoList, responseBasicData.ID)
	} else {
		_, err := p.Adapter.CallResponseCallback(nil, responseProtoMsg)
		if err != nil {
			log.Logger.Warnf("Protocol->HandleResponseData:\nCallResponseCallback: error %v", err)
		}
		log.Logger.Debugf("Protocol->HandleResponseData: failed to locate request info for responseBasicData: %v", responseBasicData)
	}

	log.Logger.Debugf("Protocol->HandleResponseData end")
	return nil
}

func (p *Protocol) GenRequestInfo(
	sigPubkey string,
	dataList ...any) (string, protoreflect.ProtoMessage, []byte, error) {
	log.Logger.Debugf("Protocol->GenRequestInfo begin:\nuserPubkey:%s\ndataList:%v\nrequestPID:%v",
		sigPubkey, dataList, p.Adapter.GetRequestPID())
	requestBasicData := NewBasicData(p.Host, sigPubkey, p.Adapter.GetRequestPID())
	requestProtoMsg, err := p.Adapter.InitRequest(requestBasicData, dataList...)
	if err != nil {
		log.Logger.Errorf("Protocol->GenRequestInfo: InitRequest error: %v", err)
		return "", nil, nil, err
	}
	requestProtoData, err := proto.Marshal(requestProtoMsg)
	if err != nil {
		log.Logger.Errorf("Protocol->GenRequestInfo: Marshal error: %v", err)
		return "", nil, nil, err
	}
	sig, err := p.Service.GetUserSig(requestProtoData)
	if err != nil {
		log.Logger.Errorf("Protocol->GenRequestInfo: GetUserSig error: %v", err)
		return "", nil, nil, err
	}
	err = p.Adapter.SetRequestSig(requestProtoMsg, sig)
	if err != nil {
		log.Logger.Errorf("Protocol->GenRequestInfo: SetRequestSig error: %v", err)
		return "", nil, nil, err
	}

	requestProtoData, err = proto.Marshal(requestProtoMsg)
	if err != nil {
		log.Logger.Errorf("Protocol->GenRequestInfo: Marshal error: %v", err)
		return "", nil, nil, err
	}

	p.RequestInfoList[requestBasicData.ID] = &RequestInfo{
		ProtoMessage:    requestProtoMsg,
		CreateTimestamp: requestBasicData.TS,
		ResponseChan:    make(chan any),
	}

	log.Logger.Debugf("Protocol->GenRequestInfo end")
	return requestBasicData.ID, requestProtoMsg, requestProtoData, nil
}

func (p *Protocol) TickCleanRequest() {
	ticker := time.NewTicker(30 * time.Minute)
	for {
		select {
		case <-ticker.C:
			for id, v := range p.RequestInfoList {
				if time.Since(time.Unix(v.CreateTimestamp, 0)) > 10*time.Minute {
					close(p.RequestInfoList[id].ResponseChan)
					delete(p.RequestInfoList, id)
				}
			}
			log.Logger.Debug("Protocol->TickCleanRequest: clean request data")
		case <-p.Ctx.Done():
			err := p.Ctx.Err()
			if err != nil {
				log.Logger.Errorf("Protocol->TickCleanRequest: %v", err)
				return
			}
		}
	}
}
