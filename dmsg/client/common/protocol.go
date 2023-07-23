package common

import (
	"fmt"
	"time"

	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (p *Protocol) HandleRequestData(requestProtoData []byte) error {
	dmsgLog.Logger.Debugf("Protocol->HandleRequestData begin\nrequestPId: %v", p.Adapter.GetRequestPID())

	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("Protocol->HandleRequestData: recovered from: err: %v", r)
		}
	}()

	err := proto.Unmarshal(requestProtoData, p.RequestProtoMsg)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: unmarshal request error: %v", err)
		return err
	}

	dmsgLog.Logger.Debugf("Protocol->HandleRequestData: protocolRequest: %v", p.RequestProtoMsg)

	requestBasicData := p.Adapter.GetRequestBasicData()
	valid := protocol.AuthProtocolMsg(p.RequestProtoMsg, requestBasicData)
	if !valid {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: failed to authenticate message")
		return fmt.Errorf("Protocol->HandleRequestData: failed to authenticate message")
	}

	callbackData, err := p.Adapter.CallRequestCallback()
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: CallRequestCallback error: %v", err)
		return err
	}
	if callbackData != nil {
		dmsgLog.Logger.Debugf("Protocol->HandleRequestData: callbackData: %v", callbackData)
	}

	// generate response message
	srcUserPubKey := p.Service.GetCurSrcUserPubKeyHex()
	responseBasicData, err := protocol.NewBasicData(p.Host, srcUserPubKey, p.Adapter.GetResponsePID())
	responseBasicData.ID = requestBasicData.ID
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: NewBasicData error: %v", err)
		return err
	}

	err = p.Adapter.InitResponse(responseBasicData, callbackData)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: InitResponse error: %v", err)
		return err
	}

	// sign the data
	protoData, err := proto.Marshal(p.ResponseProtoMsg)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: marshal response error: %v", err)
		return err
	}
	sig, err := p.Service.GetCurSrcUserSig(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: GetCurSrcUserSig error: %v", err)
		return err
	}
	err = p.Adapter.SetResponseSig(sig)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: SetResponseSig error: %v", err)
		return err
	}
	dmsgLog.Logger.Debugf("Protocol->HandleRequestData: protocolResponse: %v", p.ResponseProtoMsg)

	dmsgLog.Logger.Debugf("Protocol->HandleRequestData end")
	return nil
}

func (p *Protocol) HandleResponseData(responseProtoData []byte) error {
	dmsgLog.Logger.Debugf("Protocol->HandleResponseData begin:\nrequestPId:%s", p.Adapter.GetRequestPID())
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("Protocol->HandleResponseData: recovered from: r: %v", r)
		}
	}()

	err := proto.Unmarshal(responseProtoData, p.ResponseProtoMsg)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleResponseData: unmarshal responseProtoMsg error %v", err)
		return err
	}

	dmsgLog.Logger.Debugf("Protocol->HandleResponseData: ResponseProtoMsg: %v", p.ResponseProtoMsg)

	responseBasicData := p.Adapter.GetResponseBasicData()
	valid := protocol.AuthProtocolMsg(p.ResponseProtoMsg, responseBasicData)
	if !valid {
		dmsgLog.Logger.Errorf("Protocol->HandleResponseData: failed to authenticate message, responseProtoMsg: %v", p.ResponseProtoMsg)
		return fmt.Errorf("Protocol->HandleResponseData: failed to authenticate message, responseProtoMsg: %v", p.ResponseProtoMsg)
	}

	requestInfo, ok := p.RequestInfoList[responseBasicData.ID]
	if ok {
		callbackData, err := p.Adapter.CallResponseCallback(requestInfo.ProtoMessage, p.ResponseProtoMsg)
		if err != nil {
			dmsgLog.Logger.Warnf("Protocol->HandleResponseData: CallResponseCallback: error %v", err)
		}
		if callbackData != nil {
			dmsgLog.Logger.Debugf("Protocol->HandleResponseData: callbackData %v", callbackData)
		}
		delete(p.RequestInfoList, responseBasicData.ID)
	} else {
		dmsgLog.Logger.Warnf("Protocol->HandleResponseData: failed to locate request info for responseBasicData: %v", responseBasicData)
	}

	dmsgLog.Logger.Debugf("Protocol->HandleResponseData end")
	return nil
}

func (p *Protocol) GenRequestInfo(
	userPubkey string,
	dataList ...any) (string, protoreflect.ProtoMessage, []byte, error) {
	dmsgLog.Logger.Debugf("Protocol->GenRequestInfo begin:\nuserPubkey:%s\ndataList:%v\nrequestPID:%v", userPubkey, dataList, p.Adapter.GetRequestPID())
	requestBasicData, err := protocol.NewBasicData(p.Host, userPubkey, p.Adapter.GetRequestPID())
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->GenRequestInfo: NewBasicData error: %v", err)
		return "", nil, nil, err
	}
	err = p.Adapter.InitRequest(requestBasicData, dataList...)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->GenRequestInfo: InitRequest error: %v", err)
		return "", nil, nil, err
	}
	requestProtoData, err := proto.Marshal(p.RequestProtoMsg)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->GenRequestInfo: Marshal error: %v", err)
		return "", nil, nil, err
	}
	sig, err := p.Service.GetCurSrcUserSig(requestProtoData)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->GenRequestInfo: GetCurSrcUserSig error: %v", err)
		return "", nil, nil, err
	}
	err = p.Adapter.SetRequestSig(sig)
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->GenRequestInfo: SetRequestSig error: %v", err)
		return "", nil, nil, err
	}

	p.RequestInfoList[requestBasicData.ID] = &RequestInfo{
		ProtoMessage:    p.RequestProtoMsg,
		CreateTimestamp: requestBasicData.TS,
	}

	dmsgLog.Logger.Debugf("Protocol->GenRequestInfo end")
	return requestBasicData.ID, p.RequestProtoMsg, requestProtoData, nil
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
