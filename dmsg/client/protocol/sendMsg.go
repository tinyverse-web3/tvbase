package protocol

import (
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg"
	dmsgClientCommon "github.com/tinyverse-web3/tvbase/dmsg/client/common"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/proto"
)

type SendMsgProtocol struct {
	dmsgClientCommon.PubsubProtocol
	SendMsgRequest *pb.SendMsgReq
}

func (p *SendMsgProtocol) OnRequest(pubMsg *pubsub.Message, protocolData []byte) {
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("SendMsgProtocol->OnRequest: recovered from:", r)
		}
	}()

	err := proto.Unmarshal(protocolData, p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("SendMsgProtocol->OnRequest: unmarshal protocolData error %v", err)
		return
	}

	// requestProtocolId := p.Adapter.GetRequestProtocolID()
	requestProtocolId := pb.ProtocolID_SEND_MSG_REQ
	dmsgLog.Logger.Debugf("SendMsgProtocol->OnRequest: received request from %s, topic:%s, requestProtocolId:%s,  Message:%v",
		pubMsg.ReceivedFrom, pubMsg.Topic, requestProtocolId, p.ProtocolRequest)

	sendMsgReq, ok := p.ProtocolRequest.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("SendMsgProtocol->OnRequest: failed to cast msg to *pb.SendMsgReq")
		return
	}
	basicData := sendMsgReq.BasicData
	valid, err := protocol.EcdsaAuthProtocolMsg(p.ProtocolRequest, basicData)
	if err != nil {
		dmsgLog.Logger.Warnf("SendMsgProtocol->OnRequest: authenticate message err:%v", err)
		return
	}
	if !valid {
		dmsgLog.Logger.Warn("SendMsgProtocol->OnRequest: authenticate message fail")
		return
	}

	callbackData, err := p.Callback.OnHandleSendMsgRequest(p.ProtocolRequest, protocolData)
	if err != nil {
		dmsgLog.Logger.Errorf(fmt.Sprintf("SendMsgProtocol->OnRequest: onRequest callback error %v", err))
	}
	if callbackData != nil {
		dmsgLog.Logger.Debugf("SendMsgProtocol->OnRequest: callback data: %v", callbackData)
	}

	dmsgLog.Logger.Debugf("SendMsgProtocol->OnRequest: received response from %s, topic:%s, requestProtocolId:%s,  Message:%v",
		pubMsg.ReceivedFrom, *pubMsg.Topic, requestProtocolId, p.ProtocolRequest)
}

func (p *SendMsgProtocol) Request(sendMsgData *dmsg.SendMsgData) (*pb.SendMsgReq, error) {
	dmsgLog.Logger.Debug("SendMsgProtocol->Request ...")
	srcUserPubKey := p.ProtocolService.GetCurSrcUserPubKeyHex()
	basicData, err := protocol.NewBasicData(p.Host, srcUserPubKey, sendMsgData.DestUserPubkeyHex, pb.ProtocolID_SEND_MSG_REQ)
	if err != nil {
		return nil, err
	}
	p.SendMsgRequest = &pb.SendMsgReq{
		BasicData:  basicData,
		SrcPubkey:  sendMsgData.SrcUserPubkeyHex,
		MsgContent: sendMsgData.MsgContent,
	}

	protoData, err := proto.Marshal(p.SendMsgRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("SendMsgProtocol->Request: marshal error %v", err)
		return nil, err
	}

	sign, err := p.ProtocolService.GetCurSrcUserSign(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("SendMsgProtocol->Request: get signature error %v", err)
		return nil, err
	}

	err = p.Callback.OnSendMsgBeforePublish(p.SendMsgRequest)
	if err != nil {
		return nil, err
	}

	p.SendMsgRequest.BasicData.Sign = sign
	protoData, err = proto.Marshal(p.SendMsgRequest)
	if err != nil {
		dmsgLog.Logger.Error("SendMsgProtocol->Request: marshal protocolData error %v", err)
		return nil, err
	}

	err = p.ProtocolService.PublishProtocol(p.SendMsgRequest.BasicData.ProtocolID,
		p.SendMsgRequest.BasicData.DestPubkey, protoData, dmsgClientCommon.PubsubSource.DestUser)
	if err != nil {
		dmsgLog.Logger.Error("SendMsgProtocol->Request: publish protocol error %v", err)
		return nil, err
	}

	dmsgLog.Logger.Debug("SendMsgProtocol->Request Done.")
	return p.SendMsgRequest, nil
}

func NewSendMsgProtocol(
	host host.Host,
	protocolCallback dmsgClientCommon.PubsubProtocolCallback,
	protocolService dmsgClientCommon.ProtocolService) *SendMsgProtocol {
	ret := &SendMsgProtocol{}
	ret.Host = host
	ret.Callback = protocolCallback
	ret.ProtocolService = protocolService

	ret.SendMsgRequest = &pb.SendMsgReq{}
	ret.ProtocolRequest = ret.SendMsgRequest
	return ret
}
