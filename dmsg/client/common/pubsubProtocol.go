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
	dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData begin\nrequestProtocolId: %v", p.Adapter.GetRequestProtocolID())
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: recovered from: err: %v", r)
		}
	}()

	err := proto.Unmarshal(protocolData, p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: unmarshal error: %v", err)
		return
	}

	dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData: protocolRequest: %v", p.ProtocolRequest)

	basicData := p.Adapter.GetProtocolRequestBasicData()
	valid := protocol.AuthProtocolMsg(p.ProtocolRequest, basicData)
	if !valid {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: failed to authenticate message")
		return
	}

	callbackData, err := p.Adapter.CallProtocolRequestCallback()
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: CallProtocolRequestCallback error: %v", err)
		return
	}
	if callbackData != nil {
		dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData: callbackData: %v", callbackData)
	}

	dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData end")
}

func (p *PubsubProtocol) HandleResponseData(protocolData []byte) {
	dmsgLog.Logger.Debugf("PubsubProtocol->HandleResponseData begin\nresquestProtocolId: %v", p.Adapter.GetResponseProtocolID())

	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("PubsubProtocol->HandleResponseData: recovered from: r: %v", r)
		}

		basicData := p.Adapter.GetProtocolResponseBasicData()
		if basicData == nil {
			return
		}
		_, ok := p.RequestInfoList[basicData.Id]
		if ok {
			delete(p.RequestInfoList, basicData.Id)
		} else {
			dmsgLog.Logger.Warnf("PubsubProtocol->HandleResponseData: failed to locate requests, response:%v",
				p.ProtocolResponse)
		}
	}()

	err := proto.Unmarshal(protocolData, p.ProtocolResponse)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleResponseData: unmarshal error: %v", err)
		return
	}

	dmsgLog.Logger.Debugf("PubsubProtocol->HandleResponseData: protocolResponse: %v", p.ProtocolResponse)

	basicData := p.Adapter.GetProtocolResponseBasicData()
	valid := protocol.AuthProtocolMsg(p.ProtocolResponse, basicData)
	if !valid {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleResponseData: failed to authenticate message")
		return
	}

	callbackData, err := p.Adapter.CallProtocolResponseCallback()
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleResponseData: CallProtocolResponseCallback error: %v", err)
		return
	}
	if callbackData != nil {
		dmsgLog.Logger.Debugf("PubsubProtocol->HandleResponseData: callbackData: %v", callbackData)
	}

	dmsgLog.Logger.Debugf("PubsubProtocol->HandleResponseData end")
}

func (p *PubsubProtocol) Request(
	signUserPubKey string,
	destUserPubKey string,
	dataList ...any) (any, error) {
	dmsgLog.Logger.Debugf("PubsubProtocol->Request begin:\nsignPubKey:%s\ndestUserPubKey:%s\ndataList:%v",
		signUserPubKey, destUserPubKey, dataList)
	basicData, err := protocol.NewBasicData(p.Host, signUserPubKey, destUserPubKey, p.Adapter.GetRequestProtocolID())
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request: NewBasicData error: %v", err)
		return nil, err
	}
	err = p.Adapter.InitProtocolRequest(basicData, dataList...)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request: InitProtocolRequest error: %v", err)
		return nil, err
	}
	dmsgLog.Logger.Debugf("PubsubProtocol->Request: init protocol request:\n%v", p.ProtocolRequest)

	//sign data
	protoData, err := proto.Marshal(p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request err: marshal protocolData error: %v", err)
		return nil, err
	}
	signature, err := p.ProtocolService.GetCurSrcUserSign(protoData)
	if err != nil {
		return nil, err
	}
	err = p.Adapter.SetProtocolRequestSign(signature)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request err: set protocol request sign error: %v", err)
		return nil, err
	}

	protocolData, err := proto.Marshal(p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request err: marshal protocolData error: %v", err)
		return nil, err
	}

	err = p.ProtocolService.PublishProtocol(basicData.ProtocolID, destUserPubKey, protocolData, p.Adapter.GetPubsubSource())
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request err: pubsub publish error: %v", err)
		return nil, err
	}

	p.RequestInfoList[basicData.Id] = &RequestInfo{
		ProtoMessage:    p.ProtocolRequest,
		CreateTimestamp: basicData.Timestamp,
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
