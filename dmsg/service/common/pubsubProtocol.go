package common

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/proto"
)

func (p *PubsubProtocol) HandleRequestData(protocolData []byte) error {
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: recovered from: r: %v", r)
		}
	}()

	err := proto.Unmarshal(protocolData, p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: unmarshal protocolData err: %v", err)
		return err
	}

	requestProtocolId := p.Adapter.GetRequestPID()
	dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData: requestProtocolId:%s, protocolRequest:%v",
		requestProtocolId, p.ProtocolRequest)

	requestBasicData := p.Adapter.GetRequestBasicData()
	valid := protocol.AuthProtocolMsg(p.ProtocolRequest, requestBasicData)
	if !valid {
		dmsgLog.Logger.Warnf("PubsubProtocol->HandleRequestData: failed to authenticate message")
		return fmt.Errorf("PubsubProtocol->HandleRequestData: failed to authenticate message")
	}

	callbackData, err := p.Adapter.CallRequestCallback()
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: CallRequestCallback error: %v", err)
		return err
	}
	if callbackData != nil {
		dmsgLog.Logger.Infof("PubsubProtocol->HandleRequestData: callback data: %s", callbackData)
	}

	// generate response message
	srcUserPubKey := p.ProtocolService.GetCurSrcUserPubKeyHex()
	responseBasicData := protocol.NewBasicData(p.Host, srcUserPubKey, p.Adapter.GetResponsePID())
	responseBasicData.ID = requestBasicData.ID
	err = p.Adapter.InitResponse(responseBasicData, callbackData)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: InitResponse error: %v", err)
		return err
	}

	// sign the data
	protoData, err := proto.Marshal(p.ProtocolResponse)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: marshal response error: %v", err)
		return err
	}
	sig, err := p.ProtocolService.GetCurSrcUserSig(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: GetCurSrcUserSig error: %v", err)
		return err
	}
	err = p.Adapter.SetResponseSig(sig)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: SetResponseSig error: %v", err)
		return err
	}

	protoData, err = proto.Marshal(p.ProtocolResponse)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: marshal response error: %v", err)
	}

	// send the response
	err = p.ProtocolService.PublishProtocol(responseBasicData.PID, requestBasicData.Pubkey, protoData)

	if err == nil {
		dmsgLog.Logger.Infof("PubsubProtocol->HandleRequestData: pubulish response requestProtocolId:%s, Message:%v",
			requestProtocolId, p.ProtocolResponse)
	} else {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: pubulish response requestProtocolId:%s, Message:%v, error:%v",
			requestProtocolId, p.ProtocolResponse, err)
	}
	return nil
}

func NewPubsubProtocol(
	host host.Host,
	protocolService ProtocolService,
	protocolCallback PubsubProtocolCallback,
	adapter PubsubProtocolAdapter) *PubsubProtocol {
	protocol := &PubsubProtocol{}
	protocol.Host = host
	protocol.ProtocolService = protocolService
	protocol.Callback = protocolCallback
	protocol.Adapter = adapter
	return protocol
}
