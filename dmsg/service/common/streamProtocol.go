package common

import (
	"context"
	"io"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/proto"
)

func (p *StreamProtocol) RequestHandler(stream network.Stream) {
	localPeer := stream.Conn().LocalPeer()
	remotePeer := stream.Conn().RemotePeer()
	localMultiAddr := stream.Conn().LocalMultiaddr()
	remoteMultiAddr := stream.Conn().RemoteMultiaddr()

	sreamRequestProtocolId := p.Adapter.GetStreamRequestPID()
	sreamResponseProtocolId := p.Adapter.GetStreamResponsePID()
	// requestProtocolId := p.Adapter.GetRequestPID()
	responseProtocolId := p.Adapter.GetResponsePID()

	dmsgLog.Logger.Debugf(`StreamProtocol->RequestHandler begin:
	/nLocalPeer: %s/nRemotePeer: %s/nlocalMultiAddr: %v/nremoteMultiAddr: %v
	/nsreamRequestProtocolId: %s/nsreamResponseProtocolId: %s,
	/nresponseProtocolId: %v`,
		localPeer, remotePeer, localMultiAddr, remoteMultiAddr,
		sreamRequestProtocolId, sreamResponseProtocolId,
		/*requestProtocolId,*/ responseProtocolId)

	protoData, err := io.ReadAll(stream)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->RequestHandler: io.ReadAll: error: %v", err)
		err = stream.Reset()
		if err != nil {
			dmsgLog.Logger.Errorf("StreamProtocol->RequestHandler: stream.Reset: error: %v", err)
		}
		return
	}
	defer func() {
		err = stream.Close()
		if err != nil {
			dmsgLog.Logger.Warnf("StreamProtocol->RequestHandler: stream.Close(): error %v", err)
		}
	}()
	err = p.HandleRequestData(protoData, stream)
	if err != nil {
		return
	}
	dmsgLog.Logger.Debugf("StreamProtocol->RequestHandler end")
}

func (p *StreamProtocol) HandleRequestData(requestProtoData []byte, stream network.Stream) error {
	dmsgLog.Logger.Debugf("StreamProtocol->HandleRequestData begin")
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("StreamProtocol->HandleRequestData: recovered from:", r)
		}
	}()

	request := p.Adapter.GetEmptyRequest()
	err := proto.Unmarshal(requestProtoData, request)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->HandleRequestData: unmarshal data error %v", err)
		return err
	}
	dmsgLog.Logger.Debugf("StreamProtocol->HandleRequestData:/np.Request: %v", request)

	requestBasicData := p.Adapter.GetRequestBasicData(request)
	var retCode *pb.RetCode = nil
	var requestCallbackData any
	valid := protocol.AuthProtocolMsg(request, requestBasicData)
	if !valid {
		dmsgLog.Logger.Errorf("StreamProtocol->HandleRequestData: failed to authenticate message")
		retCode = protocol.NewFailRetCode("StreamProtocol->HandleRequestData: failed to authenticate message")
	} else {
		var retCodeData any
		requestCallbackData, retCodeData, err = p.Adapter.CallRequestCallback(request)
		if err != nil {
			retCode = protocol.NewFailRetCode(err.Error())
		} else {
			var ok bool
			retCode, ok = retCodeData.(*pb.RetCode)
			if !ok {
				retCode = protocol.NewSuccRetCode()
			}
		}
	}

	// generate response message
	responseBasicData := protocol.NewBasicData(p.Host, p.ProtocolService.GetCurSrcUserPubKeyHex(), p.Adapter.GetResponsePID())
	responseBasicData.ID = requestBasicData.ID
	response, err := p.Adapter.InitResponse(request, responseBasicData, requestCallbackData, retCode)
	if err != nil {
		return err
	}
	dmsgLog.Logger.Debugf("StreamProtocol->HandleRequestData: response:/n%v", response)

	// sign the data
	requestProtoData, err = proto.Marshal(response)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->HandleRequestData: proto.Marshal: error: %v", err)
		return err
	}
	sig, err := p.ProtocolService.GetCurSrcUserSig(requestProtoData)
	if err != nil {
		return err
	}
	err = p.Adapter.SetResponseSig(response, sig)
	if err != nil {
		return err
	}

	// send response message
	err = protocol.SendProtocolMsg(p.Ctx, p.Host, stream.Conn().RemotePeer(), p.Adapter.GetStreamResponsePID(), response)
	if err != nil {
		return err
	}

	dmsgLog.Logger.Debugf("StreamProtocol->sendResponseProtocol end")
	return nil
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
