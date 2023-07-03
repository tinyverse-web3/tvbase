package protocol

import (
	"crypto/ecdsa"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/proto"
)

type SendMsgProtocol struct {
	common.PubsubProtocol
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

	callbackData, err := p.Callback.OnSendMsgResquest(p.ProtocolRequest, protocolData)
	if err != nil {
		dmsgLog.Logger.Errorf(fmt.Sprintf("SendMsgProtocol->OnRequest: onRequest callback error %v", err))
	}
	if callbackData != nil {
		dmsgLog.Logger.Debugf("SendMsgProtocol->OnRequest: callback data: %v", callbackData)
	}

	// requestProtocolId := p.Adapter.GetRequestProtocolID()
	requestProtocolId := pb.ProtocolID_SEND_MSG_REQ
	dmsgLog.Logger.Debugf("SendMsgProtocol->OnRequest: received response from %s, topic:%s, requestProtocolId:%s,  Message:%v",
		pubMsg.ReceivedFrom, *pubMsg.Topic, requestProtocolId, p.ProtocolRequest)
}

// Pre request for prepare message to send, before send, should be signed by user, so should return the final message data for sign
func (p *SendMsgProtocol) PreRequest(senderID string, receiverID string, msgContent []byte) (*pb.SendMsgReq, []byte, error) {
	dmsgLog.Logger.Info("PreRequest ...")
	basicData, err := protocol.NewBasicData(p.Host,
		receiverID, pb.ProtocolID_SEND_MSG_REQ)
	basicData.SignPubKey = []byte(senderID)

	dmsgLog.Logger.Debugln("SignPubKey is %v", basicData.SignPubKey)
	if err != nil {
		dmsgLog.Logger.Error("PreRequest: NewBasicData error %v", err)
		return nil, nil, err
	}
	requestData := &pb.SendMsgReq{
		BasicData:  basicData,
		SrcPubkey:  senderID,
		MsgContent: nil,
	}

	requestData.MsgContent = msgContent
	protocolData, err := proto.Marshal(requestData)
	if err != nil {
		dmsgLog.Logger.Error("PreRequest: marshal protocolData error %v", err)
		return nil, nil, err
	}

	dmsgLog.Logger.Info("PreRequest done.")
	return requestData, protocolData, nil
}

// Send message, the msgContent has been signed by user
func (p *SendMsgProtocol) Request(requestMsg *pb.SendMsgReq, sign []byte) error {
	dmsgLog.Logger.Info("Request ...")
	requestMsg.BasicData.Sign = sign

	// set the sign data to requeat msg, and marshal again
	protocolData, err := proto.Marshal(requestMsg)
	if err != nil {
		dmsgLog.Logger.Error("Request: marshal protocolData error %v", err)
		return err
	}
	err = p.ClientService.PublishProtocol(requestMsg.BasicData.ProtocolID,
		requestMsg.BasicData.DestPubkey, protocolData, common.PubsubSource.DestUser)
	if err != nil {
		dmsgLog.Logger.Error("Request: publish protocol error %v", err)
		return err
	}
	dmsgLog.Logger.Info("Request Done.")
	return nil
}

/*
func (p *SendMsgProtocol) Request(msgData interface{}) error {
	sendMsgData, ok := msgData.(*dmsg.SendMsgData)
	if !ok {
		dmsgLog.Logger.Errorf("SendMsgProtocol->OnRequest: failed to cast msgData to *dmsg.SendMsgData")
		return fmt.Errorf("SendMsgProtocol->OnRequest: failed to cast msgData to *dmsg.SendMsgData")
	}

	basicData, err := protocol.NewBasicData(p.Host,
		sendMsgData.DestUserPubkeyHex, pb.ProtocolID_SEND_MSG_REQ)
	basicData.SignPubKey = []byte(sendMsgData.SrcUserPubkeyHex)

	if err != nil {
		return err
	}
	p.SendMsgRequest = &pb.SendMsgReq{
		BasicData:  basicData,
		SrcPubkey:  sendMsgData.SrcUserPubkeyHex,
		MsgContent: nil,
	}

	pubkey, err := key.PubkeyFromEcdsaHex(sendMsgData.SrcUserPubkeyHex)
	if err != nil {
		return err
	}
	encryptMsgContent, err := key.EncryptWithPubkey(pubkey, sendMsgData.MsgContent)
	if err != nil {
		return err
	}
	p.SendMsgRequest.MsgContent = encryptMsgContent
	err = p.ClientService.SaveUserMsg(p.SendMsgRequest, dmsg.MsgDirection.To)
	if err != nil {
		return err
	}

	pubkey, err = key.PubkeyFromEcdsaHex(sendMsgData.DestUserPubkeyHex)
	if err != nil {
		return err
	}
	encryptMsgContent, err = key.EncryptWithPubkey(pubkey, sendMsgData.MsgContent)
	if err != nil {
		return err
	}
	p.SendMsgRequest.MsgContent = encryptMsgContent
	protocolData, err := proto.Marshal(p.SendMsgRequest)
	if err != nil {
		return err
	}

	err = p.SetProtocolRequestSign(sendMsgData.SrcUserPrikey, protocolData)
	if err != nil {
		return err
	}

	protocolData, err = proto.Marshal(p.SendMsgRequest)
	if err != nil {
		return err
	}

	err = p.ClientService.PublishProtocol(p.SendMsgRequest.BasicData.ProtocolID,
		p.SendMsgRequest.BasicData.DestPubkey, protocolData, common.PubsubSource.DestUser)
	if err != nil {
		return err
	}
	return nil
}
*/

func (p *SendMsgProtocol) SetProtocolRequestSign(prikey *ecdsa.PrivateKey, protocolData []byte) error {
	sign, err := key.Sign(prikey, protocolData)
	if err != nil {
		return err
	}
	p.SendMsgRequest.BasicData.Sign = sign
	return nil
}

func NewSendMsgProtocol(host host.Host, protocolCallback common.PubsubProtocolCallback, clientService common.ClientService) *SendMsgProtocol {
	ret := &SendMsgProtocol{}
	ret.SendMsgRequest = &pb.SendMsgReq{}
	ret.ProtocolRequest = ret.SendMsgRequest
	ret.Host = host
	ret.ClientService = clientService
	ret.Callback = protocolCallback

	ret.ClientService.RegPubsubProtocolReqCallback(pb.ProtocolID_SEND_MSG_REQ, ret)
	return ret
}
