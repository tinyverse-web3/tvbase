package basic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/common"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/log"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/util"
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
	RequestInfoList sync.Map
	Service         common.DmsgService
	Adapter         common.Adapter
}

func (p *Protocol) HandleRequestData(requestProtoData []byte, dataList ...any) (
	protoreflect.ProtoMessage,
	protoreflect.ProtoMessage, bool, error) {
	log.Logger.Debugf("Protocol->HandleRequestData begin")

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

	requestBasicData, err := util.GetBasicData(request)
	if err != nil {
		log.Logger.Errorf("Protocol->HandleRequestData: GetRequestBasicData error: %+v", err)
		return nil, nil, false, err
	}

	valid := util.AuthProtoMsg(request, requestBasicData)
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
	responseBasicData := util.NewBasicData(p.Host, userPubkeyHex, "", p.Adapter.GetResponsePID())
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

	err = util.SetBasicSig(response, sig)
	if err != nil {
		log.Logger.Errorf("Protocol->HandleRequestData: SetBasicSig error: %v", err)
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
	requestBasicData, err := util.GetBasicData(request)
	if err != nil {
		log.Logger.Errorf("Protocol->GetErrResponse: GetRequestBasicData error: %+v", err)
		return nil, err
	}

	userPubkeyHex, err := p.Service.GetUserPubkeyHex()
	if err != nil {
		log.Logger.Errorf("Protocol->GetErrResponse: GetUserPubkeyHex error: %v", err)
		return request, err
	}
	responseBasicData := util.NewBasicData(
		p.Host,
		userPubkeyHex,
		"",
		p.Adapter.GetResponsePID(),
	)
	responseBasicData.ID = requestBasicData.ID
	responseProtoMsg, err := p.Adapter.InitResponse(
		request, responseBasicData, nil, util.NewFailRetCode(responseErr.Error()))
	if err != nil {
		log.Logger.Warnf("Protocol->GetErrResponse: InitResponse error: %v", err)
	}

	err = util.SetRetCode(responseProtoMsg, -1, responseErr.Error())
	if err != nil {
		log.Logger.Warnf("Protocol->GetErrResponse: SetResponseRetCode error: %v", err)
		return responseProtoMsg, err
	}
	responseProtoData, err := proto.Marshal(responseProtoMsg)
	if err != nil {
		log.Logger.Errorf("Protocol->GetErrResponse: marshal response error: %v", err)
		return responseProtoMsg, err
	}
	sig, err := p.Service.GetUserSig(responseProtoData)
	if err != nil {
		return responseProtoMsg, err
	}

	err = util.SetBasicSig(responseProtoMsg, sig)
	if err != nil {
		log.Logger.Errorf("Protocol->GetErrResponse: SetBasicSig error: %v", err)
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

	responseBasicData, err := util.GetBasicData(responseProtoMsg)
	if err != nil {
		log.Logger.Errorf("Protocol->HandleResponseData: GetBasicData error: %+v", err)
		return fmt.Errorf("Protocol->HandleResponseData: GetBasicData error: %+v", err)
	}

	valid := util.AuthProtoMsg(responseProtoMsg, responseBasicData)
	if !valid {
		log.Logger.Errorf("Protocol->HandleResponseData:\nfailed to authenticate message, responseProtoMsg: %+v", responseProtoMsg)
		return fmt.Errorf("Protocol->HandleResponseData: failed to authenticate message, responseProtoMsg: %+v", responseProtoMsg)
	}

	var requestInfo *RequestInfo = nil
	requestInfoData, ok := p.RequestInfoList.Load(responseBasicData.ID)
	if ok {
		requestInfo = requestInfoData.(*RequestInfo)
	}
	//log.Logger.Debugf("Protocol->HandleResponseData:\nrequestInfo: %+v", requestInfo)
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
		// close(requestInfo.ResponseChan)
		// p.RequestInfoList.Delete(responseBasicData.ID)
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
	reqPubkey string,
	dataList ...any) (string, protoreflect.ProtoMessage, []byte, error) {
	log.Logger.Debugf("Protocol->GenRequestInfo begin:\nreqPubkey:%s\nrequestPID:%v",
		reqPubkey, p.Adapter.GetRequestPID())

	requestBasicData := util.NewBasicData(p.Host, reqPubkey, p.Service.GetProxyPubkey(), p.Adapter.GetRequestPID())
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

	err = util.SetBasicSig(requestProtoMsg, sig)
	if err != nil {
		log.Logger.Errorf("Protocol->GenRequestInfo: SetBasicSig error: %v", err)
		return "", nil, nil, err
	}

	requestProtoData, err = proto.Marshal(requestProtoMsg)
	if err != nil {
		log.Logger.Errorf("Protocol->GenRequestInfo: Marshal error: %v", err)
		return "", nil, nil, err
	}

	p.RequestInfoList.Store(requestBasicData.ID, &RequestInfo{
		ProtoMessage:    requestProtoMsg,
		CreateTimestamp: requestBasicData.TS,
		ResponseChan:    make(chan any),
	})

	log.Logger.Debugf("Protocol->GenRequestInfo end")
	return requestBasicData.ID, requestProtoMsg, requestProtoData, nil
}

func (p *Protocol) TickCleanRequest() {
	ticker := time.NewTicker(30 * time.Minute)
	defaultTimeout := 5 * time.Minute
	for {
		select {
		case <-ticker.C:
			keysToDelete := []string{}
			p.RequestInfoList.Range(func(k, v interface{}) bool {
				var requestInfo *RequestInfo = v.(*RequestInfo)
				if time.Since(time.Unix(requestInfo.CreateTimestamp, 0)) > defaultTimeout {
					keysToDelete = append(keysToDelete, k.(string))
				}
				return true
			})
			for id := range keysToDelete {
				p.RequestInfoList.Delete(id)
			}
			log.Logger.Debug("Protocol->TickCleanRequest: clean free request data")
		case <-p.Ctx.Done():
			err := p.Ctx.Err()
			if err != nil {
				log.Logger.Errorf("Protocol->TickCleanRequest: %v", err)
				return
			}
		}
	}
}
