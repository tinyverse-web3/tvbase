package msg

import (
	"fmt"

	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MsgBase struct {
	dmsgServiceCommon.ProxyPubsubService
}

func (d *MsgBase) GetDestUser(pubkey string) *dmsgUser.ProxyPubsub {
	return d.GetPubsub(pubkey)
}

func (d *MsgBase) IsExistDestUser(pubkey string) bool {
	return d.GetPubsub(pubkey) != nil
}

func (d *MsgBase) SubscribeDestUser(pubkey string, isListen bool) error {
	log.Debug("MsgBase->SubscribeDestUser begin\npubkey: %s", pubkey)
	err := d.SubscribePubsub(pubkey, true, isListen, false)
	if err != nil {
		return err
	}
	return nil
}

func (d *MsgBase) UnSubscribeDestUser(pubkey string) error {
	log.Debugf("MsgBase->UnSubscribeDestUser begin\npubkey: %s", pubkey)
	err := d.UnsubscribePubsub(pubkey)
	if err != nil {
		return err
	}
	log.Debug("MsgBase->unSubscribeDestUser end")
	return nil
}

func (d *MsgBase) UnsubscribeDestUserList() error {
	return d.UnsubscribePubsubList()
}

// MsgPpCallback
func (d *MsgBase) OnPubsubMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("MsgBase->OnPubsubMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Errorf("MsgBase->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
		return nil, nil, true, fmt.Errorf("MsgBase->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
	}

	direction := msg.MsgDirection.From
	if request.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		direction = msg.MsgDirection.To
		log.Debugf("MsgBase->OnPubsubMsgRequest: request.BasicData.PeerID == d.TvBase.GetHost().ID")
	} else if request.DestPubkey != d.LightUser.Key.PubkeyHex {
		log.Warnf("MsgBase->OnPubsubMsgRequest: LightUser pubkey isn't equal to destPubkey, d.LightUser.Key.PubkeyHex: %s",
			d.LightUser.Key.PubkeyHex)
	}

	pubkey := request.BasicData.Pubkey
	if request.BasicData.ProxyPubkey != "" {
		pubkey = request.BasicData.ProxyPubkey
	}

	var responseContent []byte
	var retCode *pb.RetCode
	if d.OnReceiveMsg != nil {
		message := &msg.ReceiveMsg{
			ID:         request.BasicData.ID,
			ReqPubkey:  pubkey,
			DestPubkey: request.DestPubkey,
			Content:    request.Content,
			TimeStamp:  request.BasicData.TS,
			Direction:  direction,
		}
		var err error
		responseContent, err = d.OnReceiveMsg(message)
		if err != nil {
			retCode = &pb.RetCode{
				Code:   dmsgProtocol.ErrCode,
				Result: "MsgBase->OnPubsubMsgRequest: OnMsgRequest error: " + err.Error(),
			}
		}
	} else {
		log.Warnf("MsgBase->OnPubsubMsgRequest: OnReceiveMsg is nil")
	}
	log.Debugf("MsgBase->OnPubsubMsgRequest end")
	return responseContent, retCode, false, nil
}

func (d *MsgBase) OnPubsubMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf("MsgBase->OnPubsubMsgResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Debugf("MsgBase->OnPubsubMsgResponse: fail to convert requestProtoData to *pb.MsgReq")
		// return nil, fmt.Errorf("MsgBase->OnPubsubMsgResponse: fail to convert requestProtoData to *pb.MsgReq")
	}

	response, ok := responseProtoData.(*pb.MsgRes)
	if !ok {
		log.Errorf("MsgBase->OnPubsubMsgResponse: fail to convert responseProtoData to *pb.MsgRes")
		return nil, fmt.Errorf("MsgBase->OnPubsubMsgResponse: fail to convert responseProtoData to *pb.MsgRes")
	}

	if response.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		log.Debugf("MsgBase->OnPubsubMsgResponse: request.BasicData.PeerID == d.TvBase.GetHost().ID()")
	}

	if response.RetCode.Code != 0 {
		log.Warnf("MsgBase->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("MsgBase->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
	} else {
		if d.OnResponseMsg != nil {
			reqPubkey := ""
			reqDestPubkey := ""
			reqMsgID := ""
			reqTimeStamp := int64(0)
			if request != nil {
				reqMsgID = request.BasicData.ID
				reqPubkey = request.BasicData.Pubkey
				reqDestPubkey = request.DestPubkey
				reqTimeStamp = request.BasicData.TS
			}
			message := &msg.RespondMsg{
				ReqMsgID:      reqMsgID,
				ReqPubkey:     reqPubkey,
				ReqDestPubkey: reqDestPubkey,
				ReqTimeStamp:  reqTimeStamp,
				RespMsgID:     response.BasicData.ID,
				RespPubkey:    response.BasicData.Pubkey,
				RespContent:   response.Content,
				RespTimeStamp: response.BasicData.TS,
			}
			d.OnResponseMsg(message)
		} else {
			log.Debugf("MsgBase->OnPubsubMsgResponse: OnResponseMsg is nil")
		}
	}
	log.Debugf("MsgBase->OnPubsubMsgResponse end")
	return nil, nil
}

// DmsgService
func (d *MsgBase) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	var target *dmsgUser.Target
	if d.PubsubList[pubkey] != nil {
		target = &d.PubsubList[pubkey].Target
	} else if d.LightUser.Key.PubkeyHex == pubkey {
		target = &d.LightUser.Target
	}

	if target == nil {
		log.Errorf("MsgBase->GetPublishTarget: target is nil")
		return nil, fmt.Errorf("MsgBase->GetPublishTarget: target is nil")
	}
	return target, nil
}
