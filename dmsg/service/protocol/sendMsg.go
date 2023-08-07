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
}

func (p *SendMsgProtocol) HandleRequestData(protocolData []byte) error {
	defer func() {
		if r := recover(); r != nil {
			dmsgLog.Logger.Errorf("SendMsgProtocol->HandleRequestData: recovered from:", r)
		}
	}()

	request := p.Adapter.GetEmptyRequest()
	err := proto.Unmarshal(protocolData, request)
	if err != nil {
		dmsgLog.Logger.Errorf(fmt.Sprintf("SendMsgProtocol->HandleRequestData: proto.Unmarshal request error %v", err))
		return err
	}

	requestProtocolId := pb.PID_SEND_MSG_REQ
	dmsgLog.Logger.Debugf("SendMsgProtocol->HandleRequestData: requestProtocolId:%s,  Message:%v",
		requestProtocolId, request)

	sendMsgReq, ok := request.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("SendMsgProtocol->HandleRequestData: sendMsgReq error")
		return fmt.Errorf("SendMsgProtocol->HandleRequestData: sendMsgReq error")
	}
	basicData := sendMsgReq.BasicData
	valid, err := protocol.EcdsaAuthProtocolMsg(request, basicData)
	if err != nil {
		dmsgLog.Logger.Warnf("SendMsgProtocol->HandleRequestData: authenticate message err:%v", err)
		return err
	}
	if !valid {
		dmsgLog.Logger.Warnf("SendMsgProtocol->HandleRequestData: failed to authenticate message")
		return err
	}

	_, err = p.Callback.OnSendMsgRequest(request)
	if err != nil {
		dmsgLog.Logger.Errorf(fmt.Sprintf("SendMsgProtocol->HandleRequestData: callback error %v", err))
	}

	dmsgLog.Logger.Debugf("SendMsgProtocol->HandleRequestData: requestProtocolId:%s, Message:%v",
		requestProtocolId, request)
	return nil
}

func NewSendMsgProtocol(host host.Host,
	protocolCallback dmsgServiceCommon.PubsubProtocolCallback,
	protocolService dmsgServiceCommon.ProtocolService) *SendMsgProtocol {
	ret := &SendMsgProtocol{}
	ret.Host = host
	ret.Service = protocolService
	ret.Callback = protocolCallback
	return ret
}
