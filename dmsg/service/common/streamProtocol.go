package common

import (
	"context"
	"fmt"
	"io"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/proto"
)

func (p *StreamProtocol) RequestHandler(stream network.Stream) {
	dmsgLog.Logger.Debugf("StreamProtocol->RequestHandler begin:\nLocalPeer: %s, RemotePeer: %s",
		stream.Conn().LocalPeer(), stream.Conn().RemotePeer())
	protoData, err := io.ReadAll(stream)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->RequestHandler: error: %v", err)
		err = stream.Reset()
		if err != nil {
			dmsgLog.Logger.Errorf("StreamProtocol->RequestHandler: error: %v", err)
		}
		return
	}
	defer func() {
		err = stream.Close()
		if err != nil {
			dmsgLog.Logger.Errorf("StreamProtocol->RequestHandler: error %v", err)
		}
	}()
	p.stream = stream
	p.HandleRequestData(protoData)
	dmsgLog.Logger.Debugf("StreamProtocol->RequestHandler: end")
}

func (p *StreamProtocol) HandleRequestData(protoData []byte) {
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("StreamProtocol->HandleRequestData: recovered from:", r)
		}
	}()

	err := proto.Unmarshal(protoData, p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->HandleRequestData: unmarshal data error %v", err)
		return
	}

	var callbackData interface{}
	requestBasicData := p.Adapter.GetProtocolRequestBasicData()

	valid := protocol.AuthProtocolMsg(p.ProtocolRequest, requestBasicData)
	if !valid {
		p.sendResponseProtocol(p.stream, callbackData, fmt.Errorf("failed to authenticate message"))
		return
	}

	// callback
	callbackData, err = p.Adapter.CallProtocolRequestCallback()
	if err != nil {
		p.sendResponseProtocol(p.stream, callbackData, err)
		return
	}
	if callbackData != nil {
		dmsgLog.Logger.Infof("StreamProtocol->HandleRequestData: callback data: %s", callbackData)
	}

	p.sendResponseProtocol(p.stream, callbackData, nil)

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
		dmsgLog.Logger.Errorf("StreamProtocol->sendResponseProtocol: marshal response error: %v", err)
		return
	}
	signature, err := p.ProtocolService.GetCurSrcUserSign(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->sendResponseProtocol: GetCurSrcUserSign error: %v", err)
		return
	}

	err = p.Adapter.SetProtocolResponseSign(signature)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->sendResponseProtocol: SetProtocolResponseSign error: %v", err)
		return
	}

	err = protocol.SendProtocolMsg(
		p.Ctx,
		p.Host,
		stream.Conn().RemotePeer(),
		p.Adapter.GetStreamResponseProtocolID(),
		p.ProtocolResponse,
	)
	responseProtocolId := p.Adapter.GetResponseProtocolID()
	sreamResponseProtocolId := p.Adapter.GetStreamResponseProtocolID()
	if err == nil {
		dmsgLog.Logger.Infof("StreamProtocol->sendResponseProtocol: responseProtocolId:%s, sreamResponseProtocolId:%s, Message:%v",
			responseProtocolId, sreamResponseProtocolId, p.ProtocolResponse)
	} else {
		dmsgLog.Logger.Warnf("StreamProtocol->sendResponseProtocol: err:%v/n.responseProtocolId:%s, sreamResponseProtocolId:%s, Message:%v",
			err, responseProtocolId, sreamResponseProtocolId, p.ProtocolResponse)
	}

}

func NewStreamProtocol(ctx context.Context, host host.Host, protocolService ProtocolService, protocolCallback StreamProtocolCallback, adapter StreamProtocolAdapter) *StreamProtocol {
	protocol := &StreamProtocol{}
	protocol.Ctx = ctx
	protocol.Host = host
	protocol.ProtocolService = protocolService
	protocol.Callback = protocolCallback
	protocol.Adapter = adapter
	protocol.Host.SetStreamHandler(adapter.GetStreamRequestProtocolID(), protocol.RequestHandler)
	return protocol
}
