package common

import (
	"context"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/proto"
)

func (p *PubsubProtocol) OnResponse(pubMsg *pubsub.Message, protocolData []byte) {
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("PubsubProtocol->OnResponse: recovered from:%v", r)
		}

		basicData := p.Adapter.GetProtocolResponseBasicData()
		if basicData == nil {
			return
		}
		_, ok := p.RequestInfoList[basicData.Id]
		if ok {
			delete(p.RequestInfoList, basicData.Id)
		} else {
			dmsgLog.Logger.Warnf("PubsubProtocol->OnResponse: failed to locate requests, pubMsg:%v, response:%v",
				pubMsg, p.ProtocolResponse)
		}
	}()

	err := proto.Unmarshal(protocolData, p.ProtocolResponse)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->OnResponse: unmarshal data error %v, response:%v",
			err, p.ProtocolResponse)
		return
	}

	basicData := p.Adapter.GetProtocolResponseBasicData()
	valid := protocol.AuthProtocolMsg(p.ProtocolResponse, basicData)
	if !valid {
		dmsgLog.Logger.Warnf("PubsubProtocol->OnResponse: failed to authenticate message, response:%v", p.ProtocolResponse)
		return
	}

	callbackData, err := p.Adapter.CallProtocolResponseCallback()
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->OnResponse: response %v callback happen error: %v", p.ProtocolResponse, err)
	}
	if callbackData != nil {
		dmsgLog.Logger.Debugf("callback data: %v", callbackData)
	}

	requestProtocolId := p.Adapter.GetRequestProtocolID()
	dmsgLog.Logger.Debugf("PubsubProtocol->OnResponse: received response from %s, msgId:%s, topic:%s, requestProtocolId:%s,  Message:%v",
		pubMsg.ID, pubMsg.ReceivedFrom, pubMsg.Topic, requestProtocolId, p.ProtocolRequest)
}

func (p *PubsubProtocol) Request(signUserPubKey string, destUserPubKey string, dataList ...any) error {

	dmsgLog.Logger.Debugf("PubsubProtocol->Request begin:\nsignPubKey:%s\ndestUserPubKey:%s\ndata:%v",
		signUserPubKey, destUserPubKey, dataList)
	basicData, err := protocol.NewBasicData(p.Host, signUserPubKey, destUserPubKey, p.Adapter.GetRequestProtocolID())
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request: NewBasicData error: %v", err)
		return err
	}
	err = p.Adapter.InitProtocolRequest(basicData, dataList)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request: InitProtocolRequest error: %v", err)
		return err
	}
	dmsgLog.Logger.Debugf("PubsubProtocol->Request: init protocol request: %v", p.ProtocolRequest)

	//sign data
	protoData, err := proto.Marshal(p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request err: marshal protocolData error: %v", err)
		return err
	}
	signature, err := p.ProtocolService.GetCurSrcUserSign(protoData)
	if err != nil {
		return err
	}
	err = p.Adapter.SetProtocolRequestSign(signature)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request err: set protocol request sign error: %v", err)
		return err
	}

	protocolData, err := proto.Marshal(p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request err: marshal protocolData error: %v", err)
		return err
	}

	err = p.ProtocolService.PublishProtocol(basicData.ProtocolID, destUserPubKey, protocolData, p.Adapter.GetPubsubSource())
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request err: pubsub publish error: %v", err)
		return err
	}

	p.RequestInfoList[basicData.Id] = &RequestInfo{
		ProtoMessage:    p.ProtocolRequest,
		CreateTimestamp: basicData.Timestamp,
	}
	dmsgLog.Logger.Debugf("PubsubProtocol->Request end")
	return nil
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
