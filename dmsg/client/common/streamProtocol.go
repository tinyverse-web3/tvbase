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

func (p *StreamProtocol) ResponseHandler(stream network.Stream) {
	dmsgLog.Logger.Debugf("StreamProtocol->ResponseHandler begin:\nLocalPeer: %s, RemotePeer: %s",
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
		dmsgLog.Logger.Errorf("StreamProtocol->ResponseHandler: error %v", err)
		return
	}

	err = p.HandleResponseData(protoData)
	if err != nil {
		return
	}
	dmsgLog.Logger.Debugf("StreamProtocol->ResponseHandler: end")
}

func (p *StreamProtocol) Request(
	peerID peer.ID,
	userPubkey string,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	dmsgLog.Logger.Debugf("StreamProtocol->Request begin:\npeerID:%s", peerID)
	requestInfoId, requestProtoMsg, _, err := p.GenRequestInfo(userPubkey, dataList...)
	if err != nil {
		return nil, err
	}

	protoData, err := proto.Marshal(requestProtoMsg)
	if err != nil {
		delete(p.RequestInfoList, requestInfoId)
		dmsgLog.Logger.Errorf("StreamProtocol->Request: Marshal error: %v", err)
		return nil, err
	}

	adapter, ok := p.Adapter.(StreamProtocolAdapter)
	if !ok {
		dmsgLog.Logger.Errorf("StreamProtocol->Request: adapter is not StreamProtocolAdapter")
		return nil, fmt.Errorf("StreamProtocol->Request: adapter is not StreamProtocolAdapter")
	}
	stream, err := p.Host.NewStream(p.Ctx, peerID, adapter.GetStreamRequestPID())
	if err != nil {
		delete(p.RequestInfoList, requestInfoId)
		dmsgLog.Logger.Errorf("StreamProtocol->Request: NewStream error: %v", err)
		return nil, err
	}
	writeLen, err := stream.Write(protoData)
	if err != nil {
		delete(p.RequestInfoList, requestInfoId)
		dmsgLog.Logger.Errorf("StreamProtocol->Request: Write error: %v", err)
		if err := stream.Reset(); err != nil {
			dmsgLog.Logger.Errorf("StreamProtocol->Request: Reset error: %v", err)
		}
		return nil, err
	}

	dmsgLog.Logger.Debugf("StreamProtocol->Request: write stream len: %d", writeLen)

	err = stream.Close()
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->Request: Close error: %v", err)
	}

	dmsgLog.Logger.Debugf("StreamProtocol->Request end")
	return requestProtoMsg, nil
}

func NewStreamProtocol(
	ctx context.Context,
	host host.Host,
	protocolCallback StreamProtocolCallback,
	protocolService ProtocolService,
	adapter StreamProtocolAdapter) *StreamProtocol {
	protocol := &StreamProtocol{}
	protocol.Host = host
	protocol.Ctx = ctx
	protocol.RequestInfoList = make(map[string]*RequestInfo)
	protocol.Callback = protocolCallback
	protocol.Service = protocolService
	protocol.Adapter = adapter
	protocol.Host.SetStreamHandler(adapter.GetStreamResponsePID(), protocol.ResponseHandler)
	go protocol.TickCleanRequest()
	return protocol
}
