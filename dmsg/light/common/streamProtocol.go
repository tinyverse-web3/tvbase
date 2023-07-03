package common

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/protoio"
)

func (p *StreamProtocol) OnResponse(stream network.Stream) {
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("StreamProtocol->OnResponse: recovered from:", r)
		}
	}()

	reader := protoio.NewFullReader(stream)
	err := reader.ReadMsg(p.ProtocolResponse)
	stream.Close()

	if err != nil {
		dmsgLog.Logger.Errorf("StreamProtocol->OnResponse: onRequest read msg error %v, response:%v",
			err, p.ProtocolResponse)
		return
	}

	basicData := p.Adapter.GetProtocolResponseBasicData()
	valid := dmsgProtocol.AuthProtocolMsg(p.ProtocolResponse, basicData, true)
	if !valid {
		dmsgLog.Logger.Errorf("StreamProtocol->OnResponse: failed to authenticate message, response: %v", p.ProtocolResponse)
		return
	}

	_, ok := p.RequestInfoList[basicData.Id]
	if ok {
		delete(p.RequestInfoList, basicData.Id)
	} else {
		dmsgLog.Logger.Warnf("StreamProtocol->OnResponse: failed to locate request data object for response:%v", p.ProtocolResponse)
	}

	callbackData, err := p.Adapter.CallProtocolResponseCallback()
	if err != nil {
		dmsgLog.Logger.Warnf("StreamProtocol->OnResponse: OnCreateMailboxResponse error %v, response:%v, callbackData:%v",
			err, p.ProtocolResponse, callbackData)
	}
	if callbackData != nil {
		dmsgLog.Logger.Debugf("StreamProtocol->OnResponse: callbackData %v", callbackData)
	}

	requestProtocolId := p.Adapter.GetRequestProtocolID()
	sreamRequestProtocolId := p.Adapter.GetStreamRequestProtocolID()
	dmsgLog.Logger.Debugf("StreamProtocol->OnResponse: %s: Received response from %s. requestProtocolId:%s, sreamRequestProtocolId:%s, Message:%v",
		stream.Conn().LocalPeer(), stream.Conn().RemotePeer(), requestProtocolId, sreamRequestProtocolId, p.ProtocolRequest)
}

func (p *StreamProtocol) Request(peerId peer.ID, userPubkey string) error {
	protocolID := p.Adapter.GetRequestProtocolID()
	basicData, err := dmsgProtocol.NewBasicData(p.Host, userPubkey, protocolID)
	if err != nil {
		return err
	}

	p.Adapter.InitProtocolRequest(basicData)
	signature, err := dmsgProtocol.SignProtocolMsg(p.ProtocolRequest, p.Host)
	if err != nil {
		return err
	}
	p.Adapter.SetProtocolRequestSign(signature)

	err = dmsgProtocol.SendProtocolMsg(p.Ctx, peerId, p.Adapter.GetStreamRequestProtocolID(), p.ProtocolRequest, p.Host)
	if err != nil {
		return err
	}

	p.RequestInfoList[basicData.Id] = &RequestInfo{
		ProtoMessage:    p.ProtocolRequest,
		CreateTimestamp: basicData.Timestamp,
	}

	dmsgLog.Logger.Debugf("StreamProtocol->Request: pubsub request msg: %v", p.ProtocolRequest)
	return nil
}

func (p *StreamProtocol) RequestCustomProtocol(peerId peer.ID, userPubkey string, protocolId string, requstContent []byte) error {
	protocolID := p.Adapter.GetRequestProtocolID()
	basicData, err := dmsgProtocol.NewBasicData(p.Host, userPubkey, protocolID)
	if err != nil {
		return err
	}

	p.Adapter.InitProtocolRequest(basicData)

	err = p.Adapter.SetCustomContent(protocolId, requstContent)
	if err != nil {
		return err
	}

	signature, err := dmsgProtocol.SignProtocolMsg(p.ProtocolRequest, p.Host)
	if err != nil {
		return err
	}
	p.Adapter.SetProtocolRequestSign(signature)

	err = dmsgProtocol.SendProtocolMsg(p.Ctx, peerId, p.Adapter.GetStreamRequestProtocolID(), p.ProtocolRequest, p.Host)
	if err != nil {
		return err
	}

	p.RequestInfoList[basicData.Id] = &RequestInfo{
		ProtoMessage:    p.ProtocolRequest,
		CreateTimestamp: basicData.Timestamp,
	}

	dmsgLog.Logger.Debugf("StreamProtocol->RequestCustomProtocol: pubsub request msg: %v", p.ProtocolRequest)
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

func NewStreamProtocol(ctx context.Context, host host.Host, protocolCallback StreamProtocolCallback, adapter StreamProtocolAdapter) *StreamProtocol {
	ret := &StreamProtocol{}
	ret.Host = host
	ret.Ctx = ctx
	ret.RequestInfoList = make(map[string]*RequestInfo)
	ret.Callback = protocolCallback
	ret.Adapter = adapter
	go ret.TickCleanRequest()
	return ret
}
