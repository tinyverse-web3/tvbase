package common

import (
	"context"
	"fmt"
	"io"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (p *StreamProtocol) HandleRequestData(requestProtocolData []byte) error {
	dmsgLog.Logger.Debugf("StreamProtocol->HandleRequestData begin\nrequestPID: %v", p.Adapter.GetRequestPID())

	requestProtoMsg, responseProtoMsg, err := p.Protocol.HandleRequestData(requestProtocolData)
	if err != nil {
		if requestProtoMsg == nil {
			return err
		}
		responseProtoMsg, err = p.GetErrResponse(requestProtoMsg, err)
		if err != nil {
			return err
		}
	}

	// send the response
	adapter, ok := p.Adapter.(StreamProtocolAdapter)
	if !ok {
		dmsgLog.Logger.Errorf("StreamProtocol->HandleRequestData: adapter is not StreamProtocolAdapter")
		return fmt.Errorf("StreamProtocol->HandleRequestData: adapter is not StreamProtocolAdapter")
	}
	stream, err := p.Host.NewStream(p.Ctx, p.stream.Conn().RemotePeer(), adapter.GetStreamResponsePID())
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->HandleRequestData: NewStream error: %v", err)
		return err
	}
	responseProtoData, err := proto.Marshal(responseProtoMsg)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->HandleRequestData: marshal response error: %v", err)
		return err
	}
	writeLen, err := stream.Write(responseProtoData)
	if err != nil {
		stream.Reset()
		return err
	}
	dmsgLog.Logger.Debugf("StreamProtocol->Request: write stream len: %d", writeLen)
	err = stream.Close()
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->Request: Close error: %v", err)
	}

	dmsgLog.Logger.Debugf("StreamProtocol->HandleRequestData end")
	return nil
}

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
	err = p.HandleRequestData(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->RequestHandler: error %v", err)
		return
	}
	dmsgLog.Logger.Debugf("StreamProtocol->RequestHandler: end")
}

func (p *StreamProtocol) ResponseHandler(stream network.Stream) {
	dmsgLog.Logger.Debugf("StreamProtocol->ResponseHandler begin\nLocalPeer: %s, RemotePeer: %s",
		stream.Conn().LocalPeer(), stream.Conn().RemotePeer())
	protoData, err := io.ReadAll(stream)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->ResponseHandler: error: %v", err)
		err = stream.Reset()
		if err != nil {
			dmsgLog.Logger.Errorf("StreamProtocol->ResponseHandler: error: %v", err)
		}
		return
	}
	err = stream.Close()
	if err != nil {
		dmsgLog.Logger.Debugf("StreamProtocol->ResponseHandler: stream.Close():error: %v", err)
	}

	_ = p.HandleResponseData(protoData)
	dmsgLog.Logger.Debugf("StreamProtocol->ResponseHandler end")
}

func (p *StreamProtocol) Request(
	peerID peer.ID,
	userPubkey string,
	proxyPubkey string,
	dataList ...any) (protoreflect.ProtoMessage, chan any, error) {
	dmsgLog.Logger.Debugf("StreamProtocol->Request begin\npeerID: %s", peerID)
	requestInfoId, requestProtoMsg, _, err := p.GenRequestInfo(userPubkey, proxyPubkey, dataList...)
	if err != nil {
		return nil, nil, err
	}

	protoData, err := proto.Marshal(requestProtoMsg)
	if err != nil {
		p.RequestInfoList.Delete(requestInfoId)
		dmsgLog.Logger.Errorf("StreamProtocol->Request: Marshal error: %v", err)
		return nil, nil, err
	}

	adapter, ok := p.Adapter.(StreamProtocolAdapter)
	if !ok {
		dmsgLog.Logger.Errorf("StreamProtocol->Request: adapter is not StreamProtocolAdapter")
		return nil, nil, fmt.Errorf("StreamProtocol->Request: adapter is not StreamProtocolAdapter")
	}
	stream, err := p.Host.NewStream(p.Ctx, peerID, adapter.GetStreamRequestPID())
	if err != nil {
		p.RequestInfoList.Delete(requestInfoId)
		dmsgLog.Logger.Errorf("StreamProtocol->Request: NewStream error: %v", err)
		return nil, nil, err
	}
	writeLen, err := stream.Write(protoData)
	if err != nil {
		p.RequestInfoList.Delete(requestInfoId)
		dmsgLog.Logger.Errorf("StreamProtocol->Request: Write error: %v", err)
		if err := stream.Reset(); err != nil {
			dmsgLog.Logger.Errorf("StreamProtocol->Request: Reset error: %v", err)
		}
		return nil, nil, err
	}

	dmsgLog.Logger.Debugf("StreamProtocol->Request: write stream len: %d", writeLen)

	err = stream.Close()
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->Request: Close error: %v", err)
	}

	dmsgLog.Logger.Debugf("StreamProtocol->Request end")
	requestInfoData, _ := p.RequestInfoList.Load(requestInfoId)
	return requestProtoMsg, requestInfoData.(*RequestInfo).DoneChan, nil
}

func NewStreamProtocol(
	ctx context.Context,
	host host.Host,
	callback StreamProtocolCallback,
	service ProtocolService,
	adapter StreamProtocolAdapter) *StreamProtocol {
	protocol := &StreamProtocol{}
	protocol.Host = host
	protocol.Ctx = ctx
	protocol.Callback = callback
	protocol.Service = service
	protocol.Adapter = adapter
	protocol.Host.SetStreamHandler(adapter.GetStreamResponsePID(), protocol.ResponseHandler)
	// protocol.Host.SetStreamHandler(adapter.GetStreamRequestPID(), protocol.RequestHandler)
	go protocol.TickCleanRequest()
	return protocol
}
