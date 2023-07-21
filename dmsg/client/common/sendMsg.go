package common

// import (
// 	"fmt"

// 	"github.com/libp2p/go-libp2p/core/host"

// 	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
// 	"github.com/tinyverse-web3/tvbase/dmsg/pb"
// 	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
// 	"google.golang.org/protobuf/proto"
// )

// type SendMsgProtocol struct {
// 	PubsubProtocol
// 	SendMsgRequest *pb.SendMsgReq
// }

// func (p *SendMsgProtocol) HandleRequestData(protocolData []byte) {
// 	defer func() {
// 		if r := recover(); r != nil {
// 			dmsgLog.Logger.Errorf("SendMsgProtocol->HandleRequestData: recovered from: err: %v", r)
// 		}
// 	}()

// 	err := proto.Unmarshal(protocolData, p.ProtocolRequest)
// 	if err != nil {
// 		dmsgLog.Logger.Errorf("SendMsgProtocol->HandleRequestData: unmarshal protocolData error %v", err)
// 		return
// 	}

// 	// requestProtocolId := p.Adapter.GetRequestProtocolID()
// 	requestProtocolId := pb.ProtocolID_SEND_MSG_REQ
// 	dmsgLog.Logger.Debugf("SendMsgProtocol->HandleRequestData: requestProtocolId:%s,  Message:%v",
// 		requestProtocolId, p.ProtocolRequest)

// 	sendMsgReq, ok := p.ProtocolRequest.(*pb.SendMsgReq)
// 	if !ok {
// 		dmsgLog.Logger.Errorf("SendMsgProtocol->HandleRequestData: failed to cast msg to *pb.SendMsgReq")
// 		return
// 	}
// 	basicData := sendMsgReq.BasicData
// 	valid, err := protocol.EcdsaAuthProtocolMsg(p.ProtocolRequest, basicData)
// 	if err != nil {
// 		dmsgLog.Logger.Warnf("SendMsgProtocol->HandleRequestData: authenticate message err:%v", err)
// 		return
// 	}
// 	if !valid {
// 		dmsgLog.Logger.Warn("SendMsgProtocol->HandleRequestData: authenticate message fail")
// 		return
// 	}

// 	callbackData, err := p.Callback.OnSendMsgRequest(p.ProtocolRequest)
// 	if err != nil {
// 		dmsgLog.Logger.Errorf(fmt.Sprintf("SendMsgProtocol->HandleRequestData: callback error %v", err))
// 	}
// 	if callbackData != nil {
// 		dmsgLog.Logger.Debugf("SendMsgProtocol->HandleRequestData: callback data: %v", callbackData)
// 	}

// 	dmsgLog.Logger.Debugf("SendMsgProtocol->HandleRequestData: requestProtocolId:%s,  Message:%v",
// 		requestProtocolId, p.ProtocolRequest)
// }

// func (p *SendMsgProtocol) Request(
// 	signUserPubKey string,
// 	destUserPubKey string,
// 	dataList ...any) (any, error) {
// 	dmsgLog.Logger.Debug("SendMsgProtocol->Request begin:\nsignPubKey:%s\ndestUserPubKey:%s\ndata:%v",
// 		signUserPubKey, destUserPubKey, dataList)

// 	basicData, err := protocol.NewBasicData(p.Host, signUserPubKey, destUserPubKey, pb.ProtocolID_SEND_MSG_REQ)
// 	if err != nil {
// 		return nil, err
// 	}
// 	p.SendMsgRequest = &pb.SendMsgReq{
// 		BasicData:  basicData,
// 		SrcPubkey:  signUserPubKey,
// 		MsgContent: dataList[0].([]byte),
// 	}

// 	protoData, err := proto.Marshal(p.SendMsgRequest)
// 	if err != nil {
// 		dmsgLog.Logger.Errorf("SendMsgProtocol->Request: marshal error %v", err)
// 		return nil, err
// 	}

// 	sign, err := p.ProtocolService.GetCurSrcUserSign(protoData)
// 	if err != nil {
// 		dmsgLog.Logger.Errorf("SendMsgProtocol->Request: get signature error %v", err)
// 		return nil, err
// 	}

// 	p.SendMsgRequest.BasicData.Sign = sign
// 	protoData, err = proto.Marshal(p.SendMsgRequest)
// 	if err != nil {
// 		dmsgLog.Logger.Error("SendMsgProtocol->Request: marshal protocolData error %v", err)
// 		return nil, err
// 	}

// 	err = p.ProtocolService.PublishProtocol(p.SendMsgRequest.BasicData.ProtocolID,
// 		p.SendMsgRequest.BasicData.DestPubkey, protoData, PubsubSource.DestUser)
// 	if err != nil {
// 		dmsgLog.Logger.Error("SendMsgProtocol->Request: publish protocol error %v", err)
// 		return nil, err
// 	}

// 	dmsgLog.Logger.Debug("SendMsgProtocol->Request end")
// 	return p.SendMsgRequest, nil
// }

// func NewSendMsgProtocol(
// 	host host.Host,
// 	protocolCallback PubsubProtocolCallback,
// 	protocolService ProtocolService) *SendMsgProtocol {
// 	ret := &SendMsgProtocol{}
// 	ret.Host = host
// 	ret.Callback = protocolCallback
// 	ret.ProtocolService = protocolService

// 	ret.SendMsgRequest = &pb.SendMsgReq{}
// 	ret.ProtocolRequest = ret.SendMsgRequest
// 	return ret
// }
