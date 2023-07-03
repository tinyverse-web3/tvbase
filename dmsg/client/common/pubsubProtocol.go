package common

import (
	"context"
	"fmt"
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
	valid := protocol.AuthProtocolMsg(p.ProtocolResponse, basicData, true)
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

func (p *PubsubProtocol) Request(pubkey interface{}) error {
	userPubkey, ok := pubkey.(string)
	if !ok {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request: pubkey %v is not string", pubkey)
		return fmt.Errorf("PubsubProtocol->Request: pubic key %s is not exist", userPubkey)
	}

	protocolID := p.Adapter.GetRequestProtocolID()
	basicData, err := protocol.NewBasicData(p.Host, userPubkey, protocolID)
	if err != nil {
		return err
	}
	p.Adapter.InitProtocolRequest(basicData)

	err = p.Adapter.SetProtocolRequestSign()
	if err != nil {
		return err
	}

	protocolData, err := proto.Marshal(p.ProtocolRequest)
	if err != nil {
		return err
	}

	pubsubSource := p.Adapter.GetPubsubSource()
	err = p.ClientService.PublishProtocol(basicData.ProtocolID, userPubkey, protocolData, pubsubSource)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->Request: pubsub publish error: %v", err)
	}

	p.RequestInfoList[basicData.Id] = &RequestInfo{
		ProtoMessage:    p.ProtocolRequest,
		CreateTimestamp: basicData.Timestamp,
	}
	dmsgLog.Logger.Debugf("PubsubProtocol->Request: pubsub request msg: %v", p.ProtocolRequest)
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

func NewPubsubProtocol(ctx context.Context, host host.Host, protocolCallback PubsubProtocolCallback,
	clientService ClientService, adapter PubsubProtocolAdapter) *PubsubProtocol {
	ret := &PubsubProtocol{}
	ret.Ctx = ctx
	ret.Host = host
	ret.Callback = protocolCallback
	ret.ClientService = clientService
	ret.RequestInfoList = make(map[string]*RequestInfo)
	ret.Adapter = adapter
	go ret.TickCleanRequest()
	return ret
}
