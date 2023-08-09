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

	request := p.Adapter.GetEmptyRequest()
	err := proto.Unmarshal(protocolData, request)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: unmarshal protocolData err: %v", err)
		return err
	}

	requestProtocolId := p.Adapter.GetRequestPID()
	dmsgLog.Logger.Debugf("PubsubProtocol->HandleRequestData: requestProtocolId:%s, protocolRequest:%v",
		requestProtocolId, request)

	requestBasicData := p.Adapter.GetRequestBasicData(request)
	valid := protocol.AuthProtocolMsg(request, requestBasicData)
	if !valid {
		dmsgLog.Logger.Warnf("PubsubProtocol->HandleRequestData: failed to authenticate message")
		return fmt.Errorf("PubsubProtocol->HandleRequestData: failed to authenticate message")
	}

	requestCallbackData, retCodeData, err := p.Adapter.CallRequestCallback(request)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: CallRequestCallback error: %v", err)
		return err
	}

	// generate response message
	userPubkeyHex, err := p.Service.GetUserPubkeyHex()
	if err != nil {
		dmsgLog.Logger.Errorf("Protocol->HandleRequestData: GetUserPubkeyHex error: %+v", err)
		return err
	}

	responseBasicData := protocol.NewBasicData(p.Host, userPubkeyHex, p.Adapter.GetResponsePID())
	responseBasicData.ID = requestBasicData.ID
	response, err := p.Adapter.InitResponse(request, responseBasicData, requestCallbackData, retCodeData)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: Adapter.InitResponse error: %v", err)
		return err
	}

	// sign the data
	protoData, err := proto.Marshal(response)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: proto.marshal response error: %v", err)
		return err
	}
	sig, err := p.Service.GetUserSig(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: GetUserSig error: %v", err)
		return err
	}
	err = p.Adapter.SetResponseSig(response, sig)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: SetResponseSig error: %v", err)
		return err
	}

	protoData, err = proto.Marshal(response)
	if err != nil {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: proto.marshal response error: %v", err)
	}

	// send the response
	err = p.Service.PublishProtocol(responseBasicData.PID, requestBasicData.Pubkey, protoData)

	if err == nil {
		dmsgLog.Logger.Infof("PubsubProtocol->HandleRequestData: pubulish response requestProtocolId:%s, Message:%v",
			requestProtocolId, response)
	} else {
		dmsgLog.Logger.Errorf("PubsubProtocol->HandleRequestData: pubulish response requestProtocolId:%s, Message:%v, error:%v",
			requestProtocolId, response, err)
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
	protocol.Service = protocolService
	protocol.Callback = protocolCallback
	protocol.Adapter = adapter
	return protocol
}
