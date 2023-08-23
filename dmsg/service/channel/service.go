package channel

import (
	"fmt"
	"time"

	ipfsLog "github.com/ipfs/go-log/v2"
	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var log = ipfsLog.Logger("dmsg.service.channel")

type ChannelService struct {
	dmsgServiceCommon.ProxyPubsubService
}

func CreateService(tvbase tvbaseCommon.TvBaseService) (*ChannelService, error) {
	d := &ChannelService{}
	cfg := tvbase.GetConfig().DMsg
	err := d.Init(tvbase, cfg.MaxChannelCount, cfg.KeepChannelDay)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// sdk-common
func (d *ChannelService) Start(
	enableService bool,
	pubkeyData []byte,
	getSig dmsgKey.GetSigCallback,
	timeout time.Duration,
) error {
	log.Debug("ChannelService->Start begin")
	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()
	createPubsubProtocol := adapter.NewCreateChannelProtocol(ctx, host, d, d, enableService)
	msgProtocol := adapter.NewPubsubMsgProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(msgProtocol.Adapter.GetRequestPID(), msgProtocol)
	d.RegistPubsubProtocol(msgProtocol.Adapter.GetResponsePID(), msgProtocol)
	err := d.ProxyPubsubService.Start(enableService, pubkeyData, getSig, createPubsubProtocol, msgProtocol)
	if err != nil {
		return err
	}

	log.Debug("ChannelService->Start end")
	return nil
}

func (d *ChannelService) GetChannel(pubkey string) *dmsgUser.ProxyPubsub {
	return d.GetProxyPubsub(pubkey)
}

func (d *ChannelService) SubscribeChannel(pubkey string) error {
	log.Debugf("ChannelService->SubscribeChannel begin:\npubkey: %s", pubkey)
	err := d.SubscribePubsub(pubkey, true, true, false)
	if err != nil {
		return err
	}
	log.Debug("ChannelService->SubscribeChannel end")
	return nil
}

func (d *ChannelService) UnsubscribeChannel(pubkey string) error {
	log.Debugf("ChannelService->UnsubscribeChannel begin\npubKey: %s", pubkey)
	err := d.UnsubscribePubsub(pubkey)
	if err != nil {
		return err
	}
	log.Debug("ChannelService->UnsubscribeChannel end")
	return nil
}

func (d *ChannelService) UnsubscribeChannelList() error {
	return d.UnsubscribePubsubList()
}

// MsgPpCallback
func (d *ChannelService) OnPubsubMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("ChannelService->OnPubsubMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Errorf("ChannelService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
		return nil, nil, true, fmt.Errorf("ChannelService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
	}

	// isSelf := request.BasicData.PeerID == d.TvBase.GetHost().ID().String()
	// if isSelf {
	// 	log.Debugf("ChannelService->OnPubsubMsgRequest: request.BasicData.PeerID == d.TvBase.GetHost().ID().String()")
	// 	return nil, nil, true, nil
	// }

	destPubkey := request.DestPubkey
	proxyPubsub := d.ProxyPubsubList[destPubkey]
	if proxyPubsub == nil && d.LightUser.Key.PubkeyHex != destPubkey {
		log.Debugf("ChannelService->OnPubsubMsgRequest: lightUser/channel pubkey is not exist")
		return nil, nil, true, fmt.Errorf("ChannelService->OnPubsubMsgRequest: lightUser/channel pubkey is not exist")
	}

	var retCode *pb.RetCode = nil
	var responseContent []byte = nil
	if d.OnMsgRequest != nil {
		srcPubkey := request.BasicData.Pubkey
		destPubkey := request.DestPubkey
		msgDirection := msg.MsgDirection.From
		var err error
		responseContent, err = d.OnMsgRequest(
			srcPubkey,
			destPubkey,
			request.Content,
			request.BasicData.TS,
			request.BasicData.ID,
			msgDirection)

		if err != nil {
			retCode = &pb.RetCode{
				Code:   dmsgProtocol.AlreadyExistCode,
				Result: "ChannelService->OnPubsubMsgRequest: " + err.Error(),
			}
		}
	} else {
		log.Errorf("ChannelService->OnPubsubMsgRequest: OnReceiveMsg is nil")
	}
	return responseContent, retCode, false, nil
}

func (d *ChannelService) OnPubsubMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf(
		"ChannelService->OnPubsubMsgResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Debugf("ChannelService->OnPubsubMsgResponse: fail to convert requestProtoData to *pb.MsgReq")
		// return nil, fmt.Errorf("ChannelService->OnPubsubMsgResponse: fail to convert requestProtoData to *pb.MsgReq")
	}

	response, ok := responseProtoData.(*pb.MsgRes)
	if !ok {
		log.Errorf("ChannelService->OnPubsubMsgResponse: fail to convert responseProtoData to *pb.MsgRes")
		return nil, fmt.Errorf("ChannelService->OnPubsubMsgResponse: fail to convert responseProtoData to *pb.MsgRes")
	}

	// if response.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
	// 	log.Debugf("ChannelService->OnCreatePubusubRequest: response.BasicData.PeerID == d.TvBase.GetHost().ID().String()")
	// 	return nil, nil
	// }

	if response.RetCode.Code != 0 {
		log.Warnf("ChannelService->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("ChannelService->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
	} else {
		if d.OnMsgResponse != nil {
			requestPubkey := ""
			requestDestPubkey := ""
			if request != nil {
				requestPubkey = request.BasicData.Pubkey
				requestDestPubkey = request.DestPubkey
			}
			d.OnMsgResponse(
				requestPubkey,
				requestDestPubkey,
				response.BasicData.Pubkey,
				response.Content,
				response.BasicData.TS,
				response.BasicData.ID,
			)
		} else {
			log.Debugf("ChannelService->OnPubsubMsgRequest: onSendMsgResponse is nil")
		}
	}
	log.Debugf("ChannelService->OnPubsubMsgResponse end")
	return nil, nil
}

func (d *ChannelService) GetPublishTarget(requestProtoData protoreflect.ProtoMessage) (*dmsgUser.Target, error) {
	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Errorf("ChannelService->GetPublishTarget: fail to convert requestProtoData to *pb.MsgReq")
		return nil, fmt.Errorf("ChannelService->GetPublishTarget: cannot convert to *pb.MsgReq")
	}

	pubkey := request.DestPubkey
	var target *dmsgUser.Target
	if d.ProxyPubsubList[pubkey] != nil {
		target = &d.ProxyPubsubList[pubkey].Target
	} else if d.LightUser.Key.PubkeyHex == pubkey {
		target = &d.LightUser.Target
	}

	if target == nil {
		log.Errorf("ChannelService->GetPublishTarget: target is nil")
		return nil, fmt.Errorf("ChannelService->GetPublishTarget: target is nil")
	}
	return target, nil
}
