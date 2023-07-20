package common

import (
	"context"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/proto"
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
		dmsgLog.Logger.Errorf("StreamProtocol->ResponseHandler: error %v, response:%v", err)
		return
	}

	p.HandleResponseData(protoData)
	dmsgLog.Logger.Debugf("StreamProtocol->ResponseHandler: end")
}

func (p *StreamProtocol) HandleResponseData(protoData []byte) {
	dmsgLog.Logger.Debugf("StreamProtocol->HandleResponseData begin:\nrequestProtocolId:%s, sreamRequestProtocolId:%s",
		p.Adapter.GetRequestProtocolID(), p.Adapter.GetStreamRequestProtocolID())
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("StreamProtocol->HandleResponseData: recovered from: r: %v", r)
		}
	}()

	err := proto.Unmarshal(protoData, p.ProtocolResponse)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->HandleResponseData: unmarshal data error %v", err)
		return
	}

	basicData := p.Adapter.GetProtocolResponseBasicData()
	valid := dmsgProtocol.AuthProtocolMsg(p.ProtocolResponse, basicData)
	if !valid {
		dmsgLog.Logger.Errorf("StreamProtocol->HandleResponseData: failed to authenticate message, response: %v", p.ProtocolResponse)
		return
	}

	_, ok := p.RequestInfoList[basicData.Id]
	if ok {
		delete(p.RequestInfoList, basicData.Id)
	} else {
		dmsgLog.Logger.Warnf("StreamProtocol->HandleResponseData: failed to locate request data object for response:%v", p.ProtocolResponse)
	}

	callbackData, err := p.Adapter.CallProtocolResponseCallback()
	if err != nil {
		dmsgLog.Logger.Warnf("StreamProtocol->HandleResponseData: OnCreateMailboxResponse error %v, response:%v, callbackData:%v",
			err, p.ProtocolResponse, callbackData)
	}
	if callbackData != nil {
		dmsgLog.Logger.Debugf("StreamProtocol->HandleResponseData: callbackData %v", callbackData)
	}

	dmsgLog.Logger.Debugf("StreamProtocol->HandleResponseData end")
}

func (p *StreamProtocol) Request(
	peerId peer.ID,
	signUserPubkey string,
	destUserPubkey string,
	dataList ...any) error {
	basicData, err := dmsgProtocol.NewBasicData(
		p.Host,
		signUserPubkey,
		destUserPubkey,
		p.Adapter.GetRequestProtocolID())
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->Request: NewBasicData error: %v", err)
		return err
	}
	err = p.Adapter.InitProtocolRequest(basicData, dataList...)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->Request: InitProtocolRequest error: %v", err)
		return err
	}
	protoData, err := proto.Marshal(p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->Request: Marshal error: %v", err)
		return err
	}
	signature, err := p.ProtocolService.GetCurSrcUserSign(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->Request: GetCurSrcUserSign error: %v", err)
		return err
	}
	p.Adapter.SetProtocolRequestSign(signature)

	err = dmsgProtocol.SendProtocolMsg(
		p.Ctx,
		p.Host,
		peerId,
		p.Adapter.GetStreamRequestProtocolID(),
		p.ProtocolRequest,
	)
	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->Request: SendProtocolMsg error: %v", err)
		return err
	}

	p.RequestInfoList[basicData.Id] = &RequestInfo{
		ProtoMessage:    p.ProtocolRequest,
		CreateTimestamp: basicData.Timestamp,
	}

	dmsgLog.Logger.Debugf("StreamProtocol->Request: request: %v", p.ProtocolRequest)
	return nil
}

func (p *StreamProtocol) TickCleanRequest() {
	ticker := time.NewTicker(30 * time.Minute)
	for {
		select {
		case <-ticker.C:
			for id, v := range p.RequestInfoList {
				if time.Since(time.Unix(v.CreateTimestamp, 0)) > 1*time.Minute {
					delete(p.RequestInfoList, id)
				}
			}
			dmsgLog.Logger.Debug("StreamProtocol->TickCleanRequest: clean request data")
		case <-p.Ctx.Done():
			err := p.Ctx.Err()
			if err != nil {
				dmsgLog.Logger.Errorf("StreamProtocol->TickCleanRequest: %v", err)
				return
			}
		}
	}
}

func NewStreamProtocol(
	ctx context.Context,
	host host.Host,
	protocolCallback StreamProtocolCallback,
	protocolService ProtocolService,
	adapter StreamProtocolAdapter) *StreamProtocol {
	streamProtocol := &StreamProtocol{}
	streamProtocol.Host = host
	streamProtocol.Ctx = ctx
	streamProtocol.RequestInfoList = make(map[string]*RequestInfo)
	streamProtocol.Callback = protocolCallback
	streamProtocol.ProtocolService = protocolService
	streamProtocol.Adapter = adapter
	streamProtocol.Host.SetStreamHandler(adapter.GetStreamResponseProtocolID(), streamProtocol.ResponseHandler)
	go streamProtocol.TickCleanRequest()
	return streamProtocol
}
