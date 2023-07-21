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

func (p *SendMsgProtocol) HandleRequestData(protocolData []byte) {
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("SendMsgProtocol->HandleRequestData: recovered from:", r)
		}
	}()

	err := proto.Unmarshal(protocolData, p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf(fmt.Sprintf("SendMsgProtocol->HandleRequestData: unmarshal protoData error %v", err))
		return
	}

	// requestProtocolId := p.Adapter.GetRequestPID()
	requestProtocolId := pb.PID_SEND_MSG_REQ
	dmsgLog.Logger.Debugf("SendMsgProtocol->HandleRequestData: requestProtocolId:%s,  Message:%v",
		requestProtocolId, p.ProtocolRequest)

	sendMsgReq, ok := p.ProtocolRequest.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("SendMsgProtocol->HandleRequestData: sendMsgReq error")
		return
	}
	basicData := sendMsgReq.BasicData
	valid, err := protocol.EcdsaAuthProtocolMsg(p.ProtocolRequest, basicData)
	if err != nil {
		dmsgLog.Logger.Warnf("SendMsgProtocol->HandleRequestData: authenticate message err:%v", err)
		return
	}
	if !valid {
		dmsgLog.Logger.Warnf("SendMsgProtocol->HandleRequestData: failed to authenticate message")
		return
	}

	callbackData, err := p.Callback.OnSendMsgRequest(p.ProtocolRequest)
	if err != nil {
		dmsgLog.Logger.Errorf(fmt.Sprintf("SendMsgProtocol->HandleRequestData: callback error %v", err))
	}
	if callbackData != nil {
		dmsgLog.Logger.Debugf("SendMsgProtocol->HandleRequestData: callback data: %v", callbackData)
	}

	dmsgLog.Logger.Debugf("SendMsgProtocol->HandleRequestData: requestProtocolId:%s,  Message:%v",
		requestProtocolId, p.ProtocolRequest)
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
