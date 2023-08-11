package protocol

import (
	"context"
	"fmt"
	"io"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type StreamProtocol struct {
	Protocol
	Callback StreamProtocolCallback
}

func (p *StreamProtocol) HandleRequestData(
	requestProtocolData []byte,
	dataList ...any) error {
	log.Logger.Debugf(
		"StreamProtocol->HandleRequestData begin\nrequestProtocolData: %v,\ndataList: %v",
		requestProtocolData, dataList)

	request, response, err := p.Protocol.HandleRequestData(requestProtocolData)
	if err != nil {
		if request == nil {
			return err
		}
		response, err = p.GetErrResponse(request, err)
		if err != nil {
			return err
		}
	}
	if request == nil {
		log.Logger.Errorf("StreamProtocol->HandleRequestData: request is nil")
		return fmt.Errorf("StreamProtocol->HandleRequestData: request is nil")
	}
	if response == nil {
		log.Logger.Errorf("StreamProtocol->HandleRequestData: response is nil")
		return fmt.Errorf("StreamProtocol->HandleRequestData: response is nil")
	}
	responseData, err := proto.Marshal(response)
	if err != nil {
		log.Logger.Errorf("StreamProtocol->HandleRequestData: proto.marshal response error: %v", err)
		return err
	}

	adapter, ok := p.Adapter.(StreamProtocolAdapter)
	if !ok {
		log.Logger.Errorf("StreamProtocol->HandleRequestData: adapter is not StreamProtocolAdapter")
		return fmt.Errorf("StreamProtocol->HandleRequestData: adapter is not StreamProtocolAdapter")
	}
	streamResponseID := adapter.GetStreamResponsePID()

	remotePeerID, ok := dataList[0].(peer.ID)
	if !ok {
		log.Logger.Errorf("StreamProtocol->HandleRequestData: dataList[0] is not peer.ID")
		return fmt.Errorf("StreamProtocol->HandleRequestData: dataList[0] is not peer.ID")
	}

	// send the response
	stream, err := p.Host.NewStream(p.Ctx, remotePeerID, streamResponseID)
	if err != nil {
		log.Logger.Errorf("StreamProtocol->HandleRequestData: NewStream error: %v", err)
		return err
	}
	writeLen, err := stream.Write(responseData)
	if err != nil {
		log.Logger.Errorf("StreamProtocol->HandleRequestData: stream.Write error: %v", err)
		stream.Reset()
		return err
	}
	log.Logger.Debugf("StreamProtocol->Request: stream.write len: %d", writeLen)
	err = stream.Close()
	if err != nil {
		log.Logger.Errorf("StreamProtocol->Request: stream.Close error: %v", err)
	}

	log.Logger.Debugf("StreamProtocol->HandleRequestData end")
	return nil
}

func (p *StreamProtocol) RequestHandler(stream network.Stream) {
	localPeer := stream.Conn().LocalPeer()
	remotePeer := stream.Conn().RemotePeer()
	localMultiAddr := stream.Conn().LocalMultiaddr()
	remoteMultiAddr := stream.Conn().RemoteMultiaddr()
	streamAdapter := p.Adapter.(StreamProtocolAdapter)
	sreamRequestProtocolId := streamAdapter.GetStreamRequestPID()
	sreamResponseProtocolId := streamAdapter.GetStreamResponsePID()
	requestProtocolId := streamAdapter.GetRequestPID()
	responseProtocolId := streamAdapter.GetResponsePID()

	log.Logger.Debugf(
		"StreamProtocol->RequestHandler begin\nLocalPeer: %s\nRemotePeer: %s\nlocalMultiAddr: %v\nremoteMultiAddr: %v\nsreamRequestProtocolId: %s\nsreamResponseProtocolId: %s,\nrequestProtocolId: %v\nresponseProtocolId: %v",
		localPeer, remotePeer, localMultiAddr, remoteMultiAddr, sreamRequestProtocolId, sreamResponseProtocolId, requestProtocolId, responseProtocolId)

	protoData, err := io.ReadAll(stream)
	if err != nil {
		log.Logger.Errorf("StreamProtocol->RequestHandler: io.ReadAll: error: %v", err)
		err = stream.Reset()
		if err != nil {
			log.Logger.Errorf("StreamProtocol->RequestHandler: stream.Reset: error: %v", err)
		}
		return
	}
	remotePeerID := stream.Conn().RemotePeer()
	err = stream.Close()
	if err != nil {
		log.Logger.Debugf("StreamProtocol->RequestHandler: stream.Close(): error: %v", err)
	}
	p.HandleRequestData(protoData, remotePeerID)
	log.Logger.Debugf("StreamProtocol->RequestHandler end")
}

func (p *StreamProtocol) ResponseHandler(stream network.Stream) {
	log.Logger.Debugf("StreamProtocol->ResponseHandler begin\nLocalPeer: %s, RemotePeer: %s",
		stream.Conn().LocalPeer(), stream.Conn().RemotePeer())
	protoData, err := io.ReadAll(stream)
	if err != nil {
		log.Logger.Errorf("StreamProtocol->ResponseHandler: error: %v", err)
		err = stream.Reset()
		if err != nil {
			log.Logger.Errorf("StreamProtocol->ResponseHandler: error: %v", err)
		}
		return
	}
	err = stream.Close()
	if err != nil {
		log.Logger.Debugf("StreamProtocol->ResponseHandler: stream.Close():error: %v", err)
	}

	_ = p.HandleResponseData(protoData)
	log.Logger.Debugf("StreamProtocol->ResponseHandler end")
}

func (p *StreamProtocol) Request(
	peerID peer.ID,
	pubkey string,
	dataList ...any) (protoreflect.ProtoMessage, chan any, error) {
	log.Logger.Debugf(
		"StreamProtocol->Request begin\npeerID: %s\npubkey: %s\ndataList: %v",
		peerID, pubkey, dataList)
	requestInfoId, requestProtoMsg, _, err := p.GenRequestInfo(pubkey, dataList...)
	if err != nil {
		return nil, nil, err
	}

	protoData, err := proto.Marshal(requestProtoMsg)
	if err != nil {
		delete(p.RequestInfoList, requestInfoId)
		log.Logger.Errorf("StreamProtocol->Request: Marshal error: %v", err)
		return nil, nil, err
	}

	adapter, ok := p.Adapter.(StreamProtocolAdapter)
	if !ok {
		log.Logger.Errorf("StreamProtocol->Request: adapter is not StreamProtocolAdapter")
		return nil, nil, fmt.Errorf("StreamProtocol->Request: adapter is not StreamProtocolAdapter")
	}
	stream, err := p.Host.NewStream(p.Ctx, peerID, adapter.GetStreamRequestPID())
	if err != nil {
		delete(p.RequestInfoList, requestInfoId)
		log.Logger.Errorf("StreamProtocol->Request: NewStream error: %v", err)
		return nil, nil, err
	}
	writeLen, err := stream.Write(protoData)
	if err != nil {
		delete(p.RequestInfoList, requestInfoId)
		log.Logger.Errorf("StreamProtocol->Request: Write error: %v", err)
		if err := stream.Reset(); err != nil {
			log.Logger.Errorf("StreamProtocol->Request: Reset error: %v", err)
		}
		return nil, nil, err
	}

	log.Logger.Debugf("StreamProtocol->Request: write stream len: %d", writeLen)

	err = stream.Close()
	if err != nil {
		log.Logger.Errorf("StreamProtocol->Request: Close error: %v", err)
	}

	log.Logger.Debugf("StreamProtocol->Request end")
	return requestProtoMsg, p.RequestInfoList[requestInfoId].DoneChan, nil
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
	protocol.RequestInfoList = make(map[string]*RequestInfo)
	protocol.Callback = callback
	protocol.Service = service
	protocol.Adapter = adapter
	protocol.Host.SetStreamHandler(adapter.GetStreamResponsePID(), protocol.ResponseHandler)
	protocol.Host.SetStreamHandler(adapter.GetStreamRequestPID(), protocol.RequestHandler)
	go protocol.TickCleanRequest()
	return protocol
}
