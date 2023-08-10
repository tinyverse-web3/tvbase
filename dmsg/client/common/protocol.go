package common

import (
	"fmt"
	"time"

	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (p *Protocol) HandleRequestData(
	requestProtoData []byte) (protoreflect.ProtoMessage, protoreflect.ProtoMessage, error) {
	dmsgLog.Logger.Debugf("Protocol->HandleRequestData begin\nrequestPId: %v", p.Adapter.GetRequestPID())

	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("Protocol->HandleRequestData: recovered from: err: %v", r)
		}
	}()

	requestProtoMsg := p.Adapter.GetEmptyRequest()
	err := proto.Unmarshal(requestProtoData, requestProtoMsg)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: unmarshal request error: %v", err)
		return nil, nil, err
	}

	dmsgLog.Logger.Debugf("Protocol->HandleRequestData: protocolRequest: %v", requestProtoMsg)

	requestBasicData := p.Adapter.GetRequestBasicData(requestProtoMsg)
	valid := protocol.AuthProtoMsg(requestProtoMsg, requestBasicData)
	if !valid {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: failed to authenticate message")
		return requestProtoMsg, nil, fmt.Errorf("Protocol->HandleRequestData: failed to authenticate message")
	}

	callbackData, err := p.Adapter.CallRequestCallback(requestProtoMsg)
	if err != nil {
		return requestProtoMsg, nil, err
	}

	// generate response message
	userPubkeyHex, err := p.Service.GetUserPubkeyHex()
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: GetUserPubkeyHex error: %+v", err)
		return requestProtoMsg, nil, err
	}

	responseBasicData := protocol.NewBasicData(p.Host, userPubkeyHex, p.Adapter.GetResponsePID())
	responseBasicData.ID = requestBasicData.ID
	responseProtoMsg, err := p.Adapter.InitResponse(responseBasicData, callbackData)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: InitResponse error: %v", err)
		return requestProtoMsg, nil, err
	}

	// sign the data
	responseProtoData, err := proto.Marshal(responseProtoMsg)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: marshal response error: %v", err)
		return requestProtoMsg, nil, err
	}
	sig, err := p.Service.GetUserSig(responseProtoData)
	if err != nil {
		return requestProtoMsg, nil, err
	}
	err = p.Adapter.SetResponseSig(responseProtoMsg, sig)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: SetResponseSig error: %v", err)
		return requestProtoMsg, nil, err
	}
	dmsgLog.Logger.Debugf("Protocol->HandleRequestData: protocolResponse: %v", responseProtoMsg)

	dmsgLog.Logger.Debugf("Protocol->HandleRequestData end")
	return requestProtoMsg, responseProtoMsg, nil
}

func (p *Protocol) GetErrResponse(
	requestProtoMsg protoreflect.ProtoMessage,
	err error) (protoreflect.ProtoMessage, error) {
	responseErr := err
	requestBasicData := p.Adapter.GetRequestBasicData(requestProtoMsg)
	userPubkeyHex, err := p.Service.GetUserPubkeyHex()
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->GetErrResponse: GetUserPubkeyHex error: %v", err)
		return requestProtoMsg, err
	}
	responseBasicData := protocol.NewBasicData(
		p.Host,
		userPubkeyHex,
		p.Adapter.GetResponsePID(),
	)
	responseBasicData.ID = requestBasicData.ID
	responseProtoMsg, err := p.Adapter.InitResponse(responseBasicData)
	if err != nil {
		dmsgLog.Logger.Warnf("Protocol->GetErrResponse: InitResponse error: %v", err)
	}
	p.Adapter.SetResponseRetCode(responseProtoMsg, -1, responseErr.Error())
	responseProtoData, err := proto.Marshal(responseProtoMsg)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->GetErrResponse: marshal response error: %v", err)
		return responseProtoMsg, err
	}
	sig, err := p.Service.GetUserSig(responseProtoData)
	if err != nil {
		return responseProtoMsg, err
	}
	err = p.Adapter.SetResponseSig(responseProtoMsg, sig)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->GetErrResponse: SetResponseSig error: %v", err)
		return responseProtoMsg, err
	}
	return responseProtoMsg, nil
}

func (p *Protocol) HandleResponseData(responseProtoData []byte) error {
	dmsgLog.Logger.Debugf("Protocol->HandleResponseData begin:\nresponsePID:%s", p.Adapter.GetResponsePID())
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("Protocol->HandleResponseData: recovered from: r: %v", r)
		}
	}()

	responseProtoMsg := p.Adapter.GetEmptyResponse()
	err := proto.Unmarshal(responseProtoData, responseProtoMsg)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleResponseData: unmarshal responseProtoMsg error: %+v", err)
		return err
	}

	dmsgLog.Logger.Debugf("Protocol->HandleResponseData:\nResponseProtoMsg: %+v", responseProtoMsg)

	responseBasicData := p.Adapter.GetResponseBasicData(responseProtoMsg)
	valid := protocol.AuthProtoMsg(responseProtoMsg, responseBasicData)
	if !valid {
		dmsgLog.Logger.Errorf("Protocol->HandleResponseData:\nfailed to authenticate message, responseProtoMsg: %+v", responseProtoMsg)
		return fmt.Errorf("Protocol->HandleResponseData: failed to authenticate message, responseProtoMsg: %+v", responseProtoMsg)
	}

	requestInfo, ok := p.RequestInfoList[responseBasicData.ID]
	if ok {
		_, err := p.Adapter.CallResponseCallback(requestInfo.ProtoMessage, responseProtoMsg)
		if err != nil {
			dmsgLog.Logger.Warnf("Protocol->HandleResponseData:\nCallResponseCallback: error %v", err)
		}
		p.RequestInfoList[responseBasicData.ID].DoneChan <- responseProtoMsg
		delete(p.RequestInfoList, responseBasicData.ID)
	} else {
		dmsgLog.Logger.Warnf("Protocol->HandleResponseData: failed to locate request info for responseBasicData: %v", responseBasicData)
	}

	dmsgLog.Logger.Debugf("Protocol->HandleResponseData end")
	return nil
}

func (p *Protocol) GenRequestInfo(
	srcUserPubkey string,
	dataList ...any) (string, protoreflect.ProtoMessage, []byte, error) {
	dmsgLog.Logger.Debugf("Protocol->GenRequestInfo begin:\nuserPubkey:%s\ndataList:%v\nrequestPID:%v",
		srcUserPubkey, dataList, p.Adapter.GetRequestPID())
	requestBasicData := protocol.NewBasicData(p.Host, srcUserPubkey, p.Adapter.GetRequestPID())
	requestProtoMsg, err := p.Adapter.InitRequest(requestBasicData, dataList...)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->GenRequestInfo: InitRequest error: %v", err)
		return "", nil, nil, err
	}
	requestProtoData, err := proto.Marshal(requestProtoMsg)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->GenRequestInfo: Marshal error: %v", err)
		return "", nil, nil, err
	}
	sig, err := p.Service.GetUserSig(requestProtoData)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->GenRequestInfo: GetUserSig error: %v", err)
		return "", nil, nil, err
	}
	err = p.Adapter.SetRequestSig(requestProtoMsg, sig)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->GenRequestInfo: SetRequestSig error: %v", err)
		return "", nil, nil, err
	}

	requestProtoData, err = proto.Marshal(requestProtoMsg)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->GenRequestInfo: Marshal error: %v", err)
		return "", nil, nil, err
	}

	p.RequestInfoList[requestBasicData.ID] = &RequestInfo{
		ProtoMessage:    requestProtoMsg,
		CreateTimestamp: requestBasicData.TS,
		DoneChan:        make(chan any),
	}

	dmsgLog.Logger.Debugf("Protocol->GenRequestInfo end")
	return requestBasicData.ID, requestProtoMsg, requestProtoData, nil
}

func (p *Protocol) TickCleanRequest() {
	ticker := time.NewTicker(30 * time.Minute)
	for {
		select {
		case <-ticker.C:
			for id, v := range p.RequestInfoList {
				if time.Since(time.Unix(v.CreateTimestamp, 0)) > 1*time.Minute {
					delete(p.RequestInfoList, id)
				}
			}
			dmsgLog.Logger.Debug("Protocol->TickCleanRequest: clean request data")
		case <-p.Ctx.Done():
			err := p.Ctx.Err()
			if err != nil {
				dmsgLog.Logger.Errorf("Protocol->TickCleanRequest: %v", err)
				return
			}
		}
	}
}
