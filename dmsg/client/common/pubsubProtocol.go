package common

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/proto"
)

func (p *PubsubProtocol) HandleRequestData(protocolData []byte) {
	dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData begin\nrequestProtocolId: %v", p.Adapter.GetRequestPID())

	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: recovered from: err: %v", r)
		}
	}()

	err := proto.Unmarshal(protocolData, p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: unmarshal request error: %v", err)
		return
	}

	dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData: protocolRequest: %v", p.ProtocolRequest)

	requestBasicData := p.Adapter.GetRequestBasicData()
	valid := protocol.AuthProtocolMsg(p.ProtocolRequest, requestBasicData)
	if !valid {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: failed to authenticate message")
		return
	}

	callbackData, err := p.Adapter.CallRequestCallback()
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: CallRequestCallback error: %v", err)
		return
	}
	if callbackData != nil {
		dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData: callbackData: %v", callbackData)
	}

	// generate response message
	srcUserPubKey := p.ProtocolService.GetCurSrcUserPubKeyHex()
	responseBasicData, err := protocol.NewBasicData(p.Host, srcUserPubKey, p.Adapter.GetResponsePID())
	responseBasicData.ID = requestBasicData.ID
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: NewBasicData error: %v", err)
		return
	}

	err = p.Adapter.InitResponse(responseBasicData, callbackData)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: InitResponse error: %v", err)
		return
	}

	// sign the data
	protoData, err := proto.Marshal(p.ProtocolResponse)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: marshal response error: %v", err)
		return
	}
	sig, err := p.ProtocolService.GetCurSrcUserSig(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: GetCurSrcUserSig error: %v", err)
		return
	}
	err = p.Adapter.SetResponseSig(sig)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: SetResponseSig error: %v", err)
		return
	}

	protoData, err = proto.Marshal(p.ProtocolResponse)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: marshal response error: %v", err)
	}

	dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData: protocolResponse: %v", p.ProtocolResponse)

	// send the response
	err = p.ProtocolService.PublishProtocol(requestBasicData.Pubkey, responseBasicData.PID, protoData)

	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: PublishProtocol error:%v", err)
	}

	dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData end")
}

func (p *PubsubProtocol) HandleResponseData(protocolData []byte) {
	dmsgLog.Logger.Debugf("PubsubProtocol->HandleResponseData begin\nresquestProtocolId: %v", p.Adapter.GetResponsePID())

	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("PubsubProtocol->HandleResponseData: recovered from: r: %v", r)
		}
	}()

	err := proto.Unmarshal(protocolData, p.ProtocolResponse)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleResponseData: unmarshal error: %v", err)
		return
	}

	dmsgLog.Logger.Debugf("PubsubProtocol->HandleResponseData: protocolResponse: %v", p.ProtocolResponse)

	responseBasicData := p.Adapter.GetResponseBasicData()
	valid := protocol.AuthProtocolMsg(p.ProtocolResponse, responseBasicData)
	if !valid {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleResponseData: failed to authenticate message")
		return
	}

	requestInfo, ok := p.RequestInfoList[responseBasicData.ID]
	if ok {
		callbackData, err := p.Adapter.CallResponseCallback(requestInfo.ProtoMessage, p.ProtocolResponse)
		if err != nil {
			dmsgLog.Logger.Warnf("PubsubProtocol->HandleResponseData: CallResponseCallback error: %v", err)
		}
		if callbackData != nil {
			dmsgLog.Logger.Debugf("PubsubProtocol->HandleResponseData: callbackData %v", callbackData)
		}
		delete(p.RequestInfoList, responseBasicData.ID)
	} else {
		dmsgLog.Logger.Warnf("PubsubProtocol->HandleResponseData: failed to locate request data object for response:%v", p.ProtocolResponse)
	}

	dmsgLog.Logger.Debugf("PubsubProtocol->HandleResponseData end")
}

func (p *PubsubProtocol) Request(
	signUserPubKey string,
	destUserPubKey string,
	dataList ...any) (any, error) {
	dmsgLog.Logger.Debugf("PubsubProtocol->Request begin:\nsignPubKey:%s\ndestUserPubKey:%s\ndataList:%v",
		signUserPubKey, destUserPubKey, dataList)
	requestBasicData, err := protocol.NewBasicData(p.Host, signUserPubKey, p.Adapter.GetRequestPID())
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request: NewBasicData error: %v", err)
		return nil, err
	}
	err = p.Adapter.InitRequest(requestBasicData, dataList...)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request: InitRequest error: %v", err)
		return nil, err
	}
	dmsgLog.Logger.Debugf("PubsubProtocol->Request: init protocol request:\n%v", p.ProtocolRequest)

	//sign data
	protoData, err := proto.Marshal(p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request err: marshal request error: %v", err)
		return nil, err
	}
	sig, err := p.ProtocolService.GetCurSrcUserSig(protoData)
	if err != nil {
		return nil, err
	}
	err = p.Adapter.SetRequestSig(sig)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request err: set protocol request sign error: %v", err)
		return nil, err
	}

	protocolData, err := proto.Marshal(p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request err: marshal request error: %v", err)
		return nil, err
	}

	err = p.ProtocolService.PublishProtocol(destUserPubKey, requestBasicData.PID, protocolData)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request: PublishProtocol error: %v", err)
		return nil, err
	}

	p.RequestInfoList[requestBasicData.ID] = &RequestInfo{
		ProtoMessage:    p.ProtocolRequest,
		CreateTimestamp: requestBasicData.TS,
	}
	dmsgLog.Logger.Debugf("PubsubProtocol->Request end")
	return p.ProtocolRequest, nil
}

func (p *PubsubProtocol) TickCleanRequest() {
	ticker := time.NewTicker(30 * time.Minute)
	for {
		select {
		case <-ticker.C:
			for id, v := range p.RequestInfoList {
				if time.Since(time.Unix(v.CreateTimestamp, 0)) > 1*time.Minute {
					delete(p.RequestInfoList, id)
				}
			}
			dmsgLog.Logger.Debug("StreamProtocol->TickCleanRequest: clean request data")
		case <-p.Ctx.Done():
			err := p.Ctx.Err()
			if err != nil {
				dmsgLog.Logger.Errorf("StreamProtocol->TickCleanRequest: %v", err)
				return
			}
		}
	}
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
	ret.ProtocolService = protocolService
	ret.RequestInfoList = make(map[string]*RequestInfo)
	ret.Adapter = adapter
	go ret.TickCleanRequest()
	return ret
}
