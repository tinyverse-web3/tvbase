package common

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/protoio"
	"google.golang.org/protobuf/proto"
)

func (p *StreamProtocol) OnRequest(stream network.Stream) {
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("StreamProtocol->OnRequest: recovered from:", r)
		}
	}()
	var callbackData interface{}
	reader := protoio.NewFullReader(stream)
	err := reader.ReadMsg(p.ProtocolRequest)
	stream.Close()

	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->OnRequest: read protoRequest error: %v", err)
		return
	}

	requestBasicData := p.Adapter.GetProtocolRequestBasicData()

	dmsgLog.Logger.Infof("StreamProtocol->OnRequest: %s:before authMsg, received request from %s, Message:%v",
		stream.Conn().LocalPeer(), stream.Conn().RemotePeer(), p.ProtocolRequest)

	valid := protocol.AuthProtocolMsg(p.ProtocolRequest, requestBasicData)
	if !valid {
		p.sendResponseProtocol(stream, callbackData, fmt.Errorf("failed to authenticate message"))
		return
	}

	// callback
	callbackData, err = p.Adapter.CallProtocolRequestCallback()
	if err != nil {
		p.sendResponseProtocol(stream, callbackData, err)
		return
	}
	if callbackData != nil {
		dmsgLog.Logger.Infof("StreamProtocol->OnRequest: callback data: %s", callbackData)
	}

	p.sendResponseProtocol(stream, callbackData, nil)

}

func (p *StreamProtocol) sendResponseProtocol(stream network.Stream, callbackData interface{}, codeErr error) {
	// generate response message
	requestBasicData := p.Adapter.GetProtocolRequestBasicData()
	protocolID := p.Adapter.GetResponseProtocolID()

	srcUserPubKey := p.ProtocolService.GetCurSrcUserPubKeyHex()
	responseBasicData, err := protocol.NewBasicData(p.Host, srcUserPubKey, requestBasicData.DestPubkey, protocolID)
	responseBasicData.Id = requestBasicData.Id
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->sendResponseProtocol: NewBasicData error: %v", err)
		return
	}

	err = p.Adapter.InitProtocolResponse(responseBasicData, callbackData)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->sendResponseProtocol: InitProtocolResponse error: %v", err)
		return
	}

	if codeErr != nil {
		errMsg := codeErr.Error()
		p.Adapter.SetProtocolResponseFailRet(errMsg)
		dmsgLog.Logger.Errorf(errMsg)
	} else {
		callbackData, err = p.Adapter.CallProtocolResponseCallback()
		if err != nil {
			calbackErr := fmt.Errorf("StreamProtocol->sendResponseProtocol: CallProtocolResponseCallback error: %v", err)
			dmsgLog.Logger.Error(calbackErr)
			p.Adapter.SetProtocolResponseFailRet(calbackErr.Error())
		}
		if callbackData != nil {
			dmsgLog.Logger.Infof("StreamProtocol->sendResponseProtocol: callback data: %s", callbackData)
		}
	}

	// sign the data
	protoData, err := proto.Marshal(p.ProtocolResponse)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->OnRequest: marshal response error: %v", err)
		return
	}
	signature, err := p.ProtocolService.GetCurSrcUserSign(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->OnRequest: GetCurSrcUserSign error: %v", err)
		return
	}

	err = p.Adapter.SetProtocolResponseSign(signature)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->OnRequest: SetProtocolResponseSign error: %v", err)
		return
	}

	err = protocol.SendProtocolMsg(p.Ctx, stream.Conn().RemotePeer(), p.Adapter.GetStreamResponseProtocolID(), p.ProtocolResponse, p.Host)
	responseProtocolId := p.Adapter.GetResponseProtocolID()
	sreamResponseProtocolId := p.Adapter.GetStreamResponseProtocolID()
	if err == nil {
		dmsgLog.Logger.Infof("StreamProtocol->OnRequest: %s:response to %s sent. responseProtocolId:%s, sreamResponseProtocolId:%s, Message:%v",
			stream.Conn().LocalPeer(), stream.Conn().RemotePeer(), responseProtocolId, sreamResponseProtocolId, p.ProtocolResponse)
	} else {
		dmsgLog.Logger.Warnf("StreamProtocol->OnRequest: %s:response to %s sent is err:%v/n.responseProtocolId:%s, sreamResponseProtocolId:%s, Message:%v",
			stream.Conn().LocalPeer(), stream.Conn().RemotePeer(), err, responseProtocolId, sreamResponseProtocolId, p.ProtocolResponse)
	}

}

func NewStreamProtocol(ctx context.Context, host host.Host, protocolCallback StreamProtocolCallback, adapter StreamProtocolAdapter) *StreamProtocol {
	protocol := &StreamProtocol{}
	protocol.Ctx = ctx
	protocol.Host = host
	protocol.Callback = protocolCallback
	protocol.Adapter = adapter
	return protocol
}
