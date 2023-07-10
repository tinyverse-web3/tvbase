package common

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/proto"
)

func (p *PubsubProtocol) OnRequest(pubMsg *pubsub.Message, protocolData []byte) {
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("PubsubProtocol->OnRequest: recovered from:", r)
		}
	}()

	err := proto.Unmarshal(protocolData, p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->OnRequest: unmarshal protocolData err: %v", err)
		return
	}

	requestProtocolId := p.Adapter.GetRequestProtocolID()
	dmsgLog.Logger.Infof("received request from %s, topic:%s, requestProtocolId:%s,  Message:%v",
		pubMsg.ReceivedFrom, pubMsg.Topic, requestProtocolId, p.ProtocolRequest)

	requestBasicData := p.Adapter.GetProtocolRequestBasicData()
	valid := protocol.AuthProtocolMsg(p.ProtocolRequest, requestBasicData)
	if !valid {
		dmsgLog.Logger.Warnf("PubsubProtocol->OnRequest: failed to authenticate message")
		return
	}

	callbackData, err := p.Adapter.CallProtocolRequestCallback()
	if err != nil {
		dmsgLog.Logger.Warnf(err.Error())
		return
	}
	if callbackData != nil {
		dmsgLog.Logger.Infof("PubsubProtocol->OnRequest: callback data: %s", callbackData)
	}

	// generate response message
	protocolID := p.Adapter.GetResponseProtocolID()
	srcUserPubKey := p.ProtocolService.GetCurSrcUserPubKeyHex()
	responseBasicData, err := protocol.NewBasicData(p.Host, srcUserPubKey, requestBasicData.DestPubkey, protocolID)
	responseBasicData.Id = requestBasicData.Id
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->OnRequest: NewBasicData error: %v", err)
		return
	}
	p.Adapter.InitProtocolResponse(responseBasicData)

	// sign the data
	protoData, err := proto.Marshal(p.ProtocolResponse)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->OnRequest: marshal response error: %v", err)
		return
	}
	signature, err := p.ProtocolService.GetCurSrcUserSign(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->OnRequest: GetCurSrcUserSign error: %v", err)
		return
	}
	err = p.Adapter.SetProtocolResponseSign(signature)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->OnRequest: SetProtocolResponseSign error: %v", err)
		return
	}

	responseProtocolData, err := proto.Marshal(p.ProtocolResponse)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->OnRequest: marshal response error: %v", err)
	}

	// send the response
	err = p.ProtocolService.PublishProtocol(responseBasicData.ProtocolID, responseBasicData.DestPubkey, responseProtocolData)

	if err == nil {
		dmsgLog.Logger.Infof("PubsubProtocol->OnRequest: pubulish response msg to %s, topic:%s, requestProtocolId:%s,  Message:%v",
			pubMsg.ReceivedFrom, pubMsg.Topic, requestProtocolId, p.ProtocolResponse)
	} else {
		dmsgLog.Logger.Errorf("PubsubProtocol->OnRequest: pubulish response msg to %s, topic:%s, requestProtocolId:%s,  Message:%v, error:%v",
			pubMsg.ReceivedFrom, pubMsg.Topic, requestProtocolId, p.ProtocolResponse, err)
	}
}

func NewPubsubProtocol(host host.Host, protocolCallback PubsubProtocolCallback, protocolService ProtocolService, adapter PubsubProtocolAdapter) *PubsubProtocol {
	protocol := &PubsubProtocol{}
	protocol.Host = host
	protocol.ProtocolService = protocolService
	protocol.Callback = protocolCallback
	protocol.Adapter = adapter
	return protocol
}
