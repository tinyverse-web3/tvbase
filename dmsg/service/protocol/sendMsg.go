package protocol

import (
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/proto"
)

type SendMsgProtocol struct {
	common.PubsubProtocol
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
		dmsgLog.Logger.Errorf(fmt.Sprintf("SendMsgProtocol->OnRequest: unmarshal protoData error %v", err))
		return
	}

	// requestProtocolId := p.Adapter.GetRequestProtocolID()
	requestProtocolId := pb.ProtocolID_SEND_MSG_REQ
	dmsgLog.Logger.Infof("SendMsgProtocol->OnRequest: received request from %s, topic:%s, requestProtocolId:%s,  Message:%v",
		pubMsg.ReceivedFrom, pubMsg.Topic, requestProtocolId, p.ProtocolRequest)

	sendMsgReq, ok := p.ProtocolRequest.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("SendMsgProtocol->OnRequest: sendMsgReq error")
		return
	}
	basicData := sendMsgReq.BasicData
	valid, err := protocol.EcdsaAuthProtocolMsg(p.ProtocolRequest, basicData)
	if err != nil {
		dmsgLog.Logger.Errorf(err.Error())
		return
	}
	if !valid {
		dmsgLog.Logger.Warnf("SendMsgProtocol->OnRequest: failed to authenticate message")
		return
	}

	callbackData, err := p.Callback.OnSendMsgResquest(p.ProtocolRequest, protocolData)
	if err != nil {
		dmsgLog.Logger.Errorf(fmt.Sprintf("SendMsgProtocol->OnRequest: onRequest callback error %v", err))
	}
	if callbackData != nil {
		dmsgLog.Logger.Debugf("SendMsgProtocol->OnRequest: callback data: %v", callbackData)
	}

	dmsgLog.Logger.Debugf("SendMsgProtocol->OnRequest: receive pubsub response msg to %s, msgId:%s, topic:%s, requestProtocolId:%s,  Message:%v",
		pubMsg.ID, pubMsg.ReceivedFrom, pubMsg.Topic, requestProtocolId, p.ProtocolResponse)
}

func NewSendMsgProtocol(host host.Host, protocolCallback common.PubsubProtocolCallback, protocolService common.ProtocolService) *SendMsgProtocol {
	ret := &SendMsgProtocol{}
	ret.SendMsgRequest = &pb.SendMsgReq{}
	ret.ProtocolRequest = ret.SendMsgRequest
	ret.Host = host
	ret.ProtocolService = protocolService
	ret.Callback = protocolCallback

	ret.ProtocolService.RegPubsubProtocolReqCallback(pb.ProtocolID_SEND_MSG_REQ, ret)
	return ret
}
