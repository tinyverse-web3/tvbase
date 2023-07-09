package protocol

import (
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg"
	dmsgCLientCommon "github.com/tinyverse-web3/tvbase/dmsg/client/common"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/proto"
)

type SendMsgProtocol struct {
	dmsgCLientCommon.PubsubProtocol
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
		dmsgLog.Logger.Errorf("SendMsgProtocol->OnRequest: unmarshal protocolData error %v", err)
		return
	}

	sendMsgReq, ok := p.ProtocolRequest.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("SendMsgProtocol->OnRequest: failed to cast msg to *pb.SendMsgReq")
		return
	}
	basicData := sendMsgReq.BasicData
	valid, err := protocol.EcdsaAuthProtocolMsg(p.ProtocolRequest, basicData)
	if err != nil {
		dmsgLog.Logger.Errorf("SendMsgProtocol->OnRequest: authenticate message err:%v", err)
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

	requestProtocolId := pb.ProtocolID_SEND_MSG_REQ
	dmsgLog.Logger.Debugf("SendMsgProtocol->OnRequest: received response from %s, topic:%s, requestProtocolId:%s,  Message:%v",
		pubMsg.ReceivedFrom, *pubMsg.Topic, requestProtocolId, p.ProtocolRequest)
}

func (p *SendMsgProtocol) Request(sendMsgData *dmsg.SendMsgData, getSigCallback dmsgCLientCommon.GetSigCallback) error {
	dmsgLog.Logger.Debug("SendMsgProtocol->Request ...")

	basicData, err := protocol.NewBasicData(p.Host, sendMsgData.DestUserPubkeyHex, pb.ProtocolID_SEND_MSG_REQ)
	basicData.SignPubKey = []byte(sendMsgData.SrcUserPubkeyHex)

	if err != nil {
		return err
	}
	p.SendMsgRequest = &pb.SendMsgReq{
		BasicData:  basicData,
		SrcPubkey:  sendMsgData.SrcUserPubkeyHex,
		MsgContent: sendMsgData.MsgContent,
	}

	protoData, err := proto.Marshal(p.SendMsgRequest)
	if err != nil {
		dmsgLog.Logger.Errorf("SendMsgProtocol->Request: marshal error %v", err)
		return err
	}
	sig, err := getSigCallback(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("SendMsgProtocol->Request: get signature error %v", err)
		return err
	}

	err = p.Callback.OnSendMsgBeforePublish(p.SendMsgRequest)
	if err != nil {
		return err
	}

	p.SendMsgRequest.BasicData.Sign = sig
	protoData, err = proto.Marshal(p.SendMsgRequest)
	if err != nil {
		dmsgLog.Logger.Error("SendMsgProtocol->Request: marshal protocolData error %v", err)
		return err
	}

	err = p.ClientService.PublishProtocol(p.SendMsgRequest.BasicData.ProtocolID,
		p.SendMsgRequest.BasicData.DestPubkey, protoData, dmsgCLientCommon.PubsubSource.DestUser)
	if err != nil {
		dmsgLog.Logger.Error("SendMsgProtocol->Request: publish protocol error %v", err)
		return err
	}

	dmsgLog.Logger.Debug("SendMsgProtocol->Request Done.")
	return nil
}

func NewSendMsgProtocol(host host.Host, protocolCallback dmsgCLientCommon.PubsubProtocolCallback, clientService dmsgCLientCommon.ClientService) *SendMsgProtocol {
	ret := &SendMsgProtocol{}
	ret.SendMsgRequest = &pb.SendMsgReq{}
	ret.ProtocolRequest = ret.SendMsgRequest
	ret.Host = host
	ret.ClientService = clientService
	ret.Callback = protocolCallback

	ret.ClientService.RegPubsubProtocolReqCallback(pb.ProtocolID_SEND_MSG_REQ, ret)
	return ret
}
