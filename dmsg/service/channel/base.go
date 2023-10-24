package channel

import (
	"fmt"

	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ChannelBase struct {
	dmsgServiceCommon.ProxyPubsubService
	enable bool
}

func (d *ChannelBase) GetChannel(pubkey string) *dmsgUser.ProxyPubsub {
	return d.GetPubsub(pubkey)
}

func (d *ChannelBase) IsExistChannel(pubkey string) bool {
	return d.GetPubsub(pubkey) != nil
}

func (d *ChannelBase) SubscribeChannel(pubkey string) error {
	log.Debugf("ChannelBase->SubscribeChannel begin:\npubkey: %s", pubkey)
	err := d.SubscribePubsub(pubkey, true, true, false)
	if err != nil {
		return err
	}
	log.Debug("ChannelBase->SubscribeChannel end")
	return nil
}

func (d *ChannelBase) UnsubscribeChannel(pubkey string) error {
	log.Debugf("ChannelBase->UnsubscribeChannel begin\npubKey: %s", pubkey)
	err := d.UnsubscribePubsub(pubkey)
	if err != nil {
		return err
	}
	log.Debug("ChannelBase->UnsubscribeChannel end")
	return nil
}

func (d *ChannelBase) UnsubscribeChannelList() error {
	return d.UnsubscribePubsubList()
}

// MsgPpCallback
func (d *ChannelBase) OnPubsubMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("ChannelBase->OnPubsubMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	if !d.enable {
		return nil, nil, true, nil
	}
	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Errorf("ChannelBase->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
		return nil, nil, true, fmt.Errorf("ChannelBase->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
	}

	// isSelf := request.BasicData.PeerID == d.TvBase.GetHost().ID().String()
	// if isSelf {
	// 	log.Debugf("ChannelBase->OnPubsubMsgRequest: request.BasicData.PeerID == d.TvBase.GetHost().ID().String()")
	// 	return nil, nil, true, nil
	// }

	proxyPubsub := d.PubsubList[request.DestPubkey]
	if proxyPubsub == nil && d.LightUser.Key.PubkeyHex != request.DestPubkey {
		log.Debugf("ChannelBase->OnPubsubMsgRequest: lightUser/channel pubkey is not exist")
		return nil, nil, true, fmt.Errorf("ChannelBase->OnPubsubMsgRequest: lightUser/channel pubkey is not exist")
	}

	var retCode *pb.RetCode
	var responseContent []byte
	if d.OnReceiveMsg != nil {
		message := &msg.ReceiveMsg{
			ID:         request.BasicData.ID,
			ReqPubkey:  request.BasicData.Pubkey,
			DestPubkey: request.DestPubkey,
			Content:    request.Content,
			TimeStamp:  request.BasicData.TS,
			Direction:  msg.MsgDirection.From,
		}
		var err error
		responseContent, err = d.OnReceiveMsg(message)

		if err != nil {
			retCode = &pb.RetCode{
				Code:   dmsgProtocol.ErrCode,
				Result: "ChannelBase->OnPubsubMsgRequest: " + err.Error(),
			}
		}
	} else {
		log.Errorf("ChannelBase->OnPubsubMsgRequest: OnReceiveMsg is nil")
	}
	return responseContent, retCode, false, nil
}

func (d *ChannelBase) OnPubsubMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf("ChannelBase->OnPubsubMsgResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	if !d.enable {
		return nil, nil
	}
	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Debugf("ChannelBase->OnPubsubMsgResponse: fail to convert requestProtoData to *pb.MsgReq")
		// return nil, fmt.Errorf("ChannelBase->OnPubsubMsgResponse: fail to convert requestProtoData to *pb.MsgReq")
	}

	response, ok := responseProtoData.(*pb.MsgRes)
	if !ok {
		log.Errorf("ChannelBase->OnPubsubMsgResponse: fail to convert responseProtoData to *pb.MsgRes")
		return nil, fmt.Errorf("ChannelBase->OnPubsubMsgResponse: fail to convert responseProtoData to *pb.MsgRes")
	}

	if response.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		log.Debugf("ChannelBase->OnPubsubMsgResponse: request.BasicData.PeerID == d.TvBase.GetHost().ID()")
	}

	if response.RetCode.Code != 0 {
		log.Warnf("ChannelBase->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("ChannelBase->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
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
			log.Debugf("ChannelBase->OnPubsubMsgRequest: onSendMsgResponse is nil")
		}
	}
	log.Debugf("ChannelBase->OnPubsubMsgResponse end")
	return nil, nil
}

func (d *ChannelBase) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	var target *dmsgUser.Target
	if d.PubsubList[pubkey] != nil {
		target = &d.PubsubList[pubkey].Target
	} else if d.LightUser.Key.PubkeyHex == pubkey {
		target = &d.LightUser.Target
	}

	if target == nil {
		log.Errorf("ChannelBase->GetPublishTarget: target is nil")
		return nil, fmt.Errorf("ChannelBase->GetPublishTarget: target is nil")
	}
	return target, nil
}
