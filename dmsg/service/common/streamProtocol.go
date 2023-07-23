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

	err := proto.Unmarshal(protoData, p.Request)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->HandleRequestData: unmarshal data error %v", err)
		return
	}

	var callbackData interface{}
	requestBasicData := p.Adapter.GetRequestBasicData()

	valid := protocol.AuthProtocolMsg(p.Request, requestBasicData)
	if !valid {
		p.sendResponseProtocol(p.stream, callbackData, fmt.Errorf("failed to authenticate message"))
		return
	}

	// callback
	callbackData, err = p.Adapter.CallRequestCallback()
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
	requestBasicData := p.Adapter.GetRequestBasicData()
	protocolID := p.Adapter.GetResponsePID()

	srcUserPubKey := p.ProtocolService.GetCurSrcUserPubKeyHex()
	responseBasicData, err := protocol.NewBasicData(p.Host, srcUserPubKey, protocolID)
	responseBasicData.ID = requestBasicData.ID
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->sendResponseProtocol: NewBasicData error: %v", err)
		return
	}

	err = p.Adapter.InitResponse(responseBasicData, callbackData)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->sendResponseProtocol: InitResponse error: %v", err)
		return
	}

	if codeErr != nil {
		errMsg := codeErr.Error()
		p.Adapter.SetProtocolResponseFailRet(errMsg)
		dmsgLog.Logger.Errorf(errMsg)
	} else {
		callbackData, err = p.Adapter.CallResponseCallback()
		if err != nil {
			calbackErr := fmt.Errorf("StreamProtocol->sendResponseProtocol: CallResponseCallback error: %v", err)
			dmsgLog.Logger.Error(calbackErr)
			p.Adapter.SetProtocolResponseFailRet(calbackErr.Error())
		}
		if callbackData != nil {
			dmsgLog.Logger.Infof("StreamProtocol->sendResponseProtocol: callback data: %s", callbackData)
		}
	}

	// sign the data
	protoData, err := proto.Marshal(p.Response)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->sendResponseProtocol: marshal response error: %v", err)
		return
	}
	signature, err := p.ProtocolService.GetCurSrcUserSig(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->sendResponseProtocol: GetCurSrcUserSig error: %v", err)
		return
	}

	err = p.Adapter.SetResponseSig(signature)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->sendResponseProtocol: SetResponseSig error: %v", err)
		return
	}

	err = protocol.SendProtocolMsg(
		p.Ctx,
		p.Host,
		stream.Conn().RemotePeer(),
		p.Adapter.GetStreamResponsePID(),
		p.Response,
	)
	responseProtocolId := p.Adapter.GetResponsePID()
	sreamResponseProtocolId := p.Adapter.GetStreamResponsePID()
	if err == nil {
		dmsgLog.Logger.Infof("StreamProtocol->sendResponseProtocol: responseProtocolId:%s, sreamResponseProtocolId:%s, Message:%v",
			responseProtocolId, sreamResponseProtocolId, p.Response)
	} else {
		dmsgLog.Logger.Warnf("StreamProtocol->sendResponseProtocol: err:%v/n.responseProtocolId:%s, sreamResponseProtocolId:%s, Message:%v",
			err, responseProtocolId, sreamResponseProtocolId, p.Response)
	}

}

func NewStreamProtocol(ctx context.Context, host host.Host, protocolService ProtocolService, protocolCallback StreamProtocolCallback, adapter StreamProtocolAdapter) *StreamProtocol {
	protocol := &StreamProtocol{}
	protocol.Ctx = ctx
	protocol.Host = host
	protocol.ProtocolService = protocolService
	protocol.Callback = protocolCallback
	protocol.Adapter = adapter
	protocol.Host.SetStreamHandler(adapter.GetStreamRequestPID(), protocol.RequestHandler)
	return protocol
}
