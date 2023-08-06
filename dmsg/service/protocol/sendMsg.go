package protocol

import (
	"fmt"

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

func (p *SendMsgProtocol) HandleRequestData(protocolData []byte) error {
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("SendMsgProtocol->HandleRequestData: recovered from:", r)
		}
	}()

	err := proto.Unmarshal(protocolData, p.Request)
	if err != nil {
		dmsgLog.Logger.Errorf(fmt.Sprintf("SendMsgProtocol->HandleRequestData: unmarshal protoData error %v", err))
		return err
	}

	// requestProtocolId := p.Adapter.GetRequestPID()
	requestProtocolId := pb.PID_SEND_MSG_REQ
	dmsgLog.Logger.Debugf("SendMsgProtocol->HandleRequestData: requestProtocolId:%s,  Message:%v",
		requestProtocolId, p.Request)

	sendMsgReq, ok := p.Request.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("SendMsgProtocol->HandleRequestData: sendMsgReq error")
		return fmt.Errorf("SendMsgProtocol->HandleRequestData: sendMsgReq error")
	}
	basicData := sendMsgReq.BasicData
	valid, err := protocol.EcdsaAuthProtocolMsg(p.Request, basicData)
	if err != nil {
		dmsgLog.Logger.Warnf("SendMsgProtocol->HandleRequestData: authenticate message err:%v", err)
		return err
	}
	if !valid {
		dmsgLog.Logger.Warnf("SendMsgProtocol->HandleRequestData: failed to authenticate message")
		return err
	}

	callbackData, err := p.Callback.OnSendMsgRequest(p.Request)
	if err != nil {
		dmsgLog.Logger.Errorf(fmt.Sprintf("SendMsgProtocol->HandleRequestData: callback error %v", err))
	}
	if callbackData != nil {
		dmsgLog.Logger.Debugf("SendMsgProtocol->HandleRequestData: callback data: %v", callbackData)
	}

	dmsgLog.Logger.Debugf("SendMsgProtocol->HandleRequestData: requestProtocolId:%s,  Message:%v",
		requestProtocolId, p.Request)
	return nil
}

func NewSendMsgProtocol(host host.Host, protocolCallback dmsgServiceCommon.PubsubProtocolCallback, protocolService dmsgServiceCommon.ProtocolService) *SendMsgProtocol {
	ret := &SendMsgProtocol{}
	ret.SendMsgRequest = &pb.SendMsgReq{}
	ret.Request = ret.SendMsgRequest
	ret.Host = host
	ret.Service = protocolService
	ret.Callback = protocolCallback
	return ret
}
