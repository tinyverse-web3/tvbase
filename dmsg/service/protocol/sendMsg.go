package protocol

import (
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/proto"
)

type SendMsgProtocol struct {
	dmsgServiceCommon.PubsubProtocol
	SendMsgRequest *pb.SendMsgReq
}

// Receive a message
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
	dmsgLog.Logger.Debugf("SendMsgProtocol->OnRequest: received request from %s, topic:%s, requestProtocolId:%s,  Message:%v",
		pubMsg.ReceivedFrom, pubMsg.Topic, requestProtocolId, p.ProtocolRequest)

	sendMsgReq, ok := p.ProtocolRequest.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("SendMsgProtocol->OnRequest: sendMsgReq error")
		return
	}
	basicData := sendMsgReq.BasicData
	valid, err := protocol.EcdsaAuthProtocolMsg(p.ProtocolRequest, basicData)
	if err != nil {
		dmsgLog.Logger.Warnf("SendMsgProtocol->OnRequest: authenticate message err:%v", err)
		return
	}
	if !valid {
		dmsgLog.Logger.Warnf("SendMsgProtocol->OnRequest: failed to authenticate message")
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

func NewSendMsgProtocol(host host.Host, protocolCallback dmsgServiceCommon.PubsubProtocolCallback, protocolService dmsgServiceCommon.ProtocolService) *SendMsgProtocol {
	ret := &SendMsgProtocol{}
	ret.SendMsgRequest = &pb.SendMsgReq{}
	ret.ProtocolRequest = ret.SendMsgRequest
	ret.Host = host
	ret.ProtocolService = protocolService
	ret.Callback = protocolCallback
	return ret
}
