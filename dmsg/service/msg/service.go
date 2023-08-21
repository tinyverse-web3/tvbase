package msg

import (
	"fmt"

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

var log = ipfsLog.Logger("dmsg.service.msg")

type MsgService struct {
	dmsgServiceCommon.ProxyPubsubService
}

func CreateService(tvbaseService tvbaseCommon.TvBaseService) (*MsgService, error) {
	d := &MsgService{}
	cfg := d.GetConfig()
	err := d.Init(tvbaseService, cfg.MaxMsgCount, cfg.KeepMsgDay)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// sdk-common
func (d *MsgService) Start(enableService bool, pubkeyData []byte, getSig dmsgKey.GetSigCallback) error {
	log.Debugf("MsgService->Start begin\nenableService: %v", enableService)
	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()
	createChannelProtocol := adapter.NewCreateChannelProtocol(ctx, host, d, d)
	pubsubMsgProtocol := adapter.NewPubsubMsgProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(pubsubMsgProtocol.Adapter.GetRequestPID(), pubsubMsgProtocol)
	d.RegistPubsubProtocol(pubsubMsgProtocol.Adapter.GetResponsePID(), pubsubMsgProtocol)
	err := d.ProxyPubsubService.Start(enableService, pubkeyData, getSig, createChannelProtocol, pubsubMsgProtocol)
	if err != nil {
		return err
	}

	err = d.HandlePubsubProtocol(&d.LightUser.Target)
	if err != nil {
		log.Errorf("MsgService->Start: HandlePubsubProtocol error: %v", err)
		return err
	}

	log.Debug("MsgService->Start end")
	return nil
}

func (d *MsgService) GetDestUser(pubkey string) *dmsgUser.ProxyPubsub {
	return d.GetProxyPubsub(pubkey)
}

func (d *MsgService) SubscribeDestUser(pubkey string) error {
	log.Debug("MsgService->SubscribeDestUser begin\npubkey: %s", pubkey)
	err := d.SubscribePubsub(pubkey, true, false)
	if err != nil {
		return err
	}
	return nil
}

func (d *MsgService) UnsubscribeDestUser(pubkey string) error {
	log.Debugf("MsgService->UnSubscribeDestUser begin\npubkey: %s", pubkey)
	err := d.UnsubscribePubsub(pubkey)
	if err != nil {
		return err
	}
	log.Debug("MsgService->unSubscribeDestUser end")
	return nil
}

func (d *MsgService) UnsubscribeDestUserList() error {
	return d.UnsubscribePubsubList()
}

// MsgPpCallback
func (d *MsgService) OnPubsubMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("MsgService->OnPubsubMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Errorf("MsgService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
		return nil, nil, true, fmt.Errorf("MsgService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
	}
	if request.DestPubkey != d.LightUser.Key.PubkeyHex {
		log.Errorf("MsgService->OnPubsubMsgRequest: request.DestPubkey != d.LightUser.Key.PubkeyHex")
		return nil, nil, true, fmt.Errorf("MsgService->OnPubsubMsgRequest: request.DestPubkey != d.LightUser.Key.PubkeyHex")
	}

	destPubkey := request.DestPubkey
	if d.LightUser.Key.PubkeyHex != destPubkey {
		log.Debugf(
			"ChannelService->OnPubsubMsgRequest: LightUser pubkey isn't equal to destPubkey, d.LightUser.Key.PubkeyHex: %s",
			d.LightUser.Key.PubkeyHex)
		return nil, nil, true,
			fmt.Errorf(
				"ChannelService->OnPubsubMsgRequest: LightUser pubkey isn't equal to destPubkey, d.LightUser.Key.PubkeyHex: %s",
				d.LightUser.Key.PubkeyHex)
	}

	if d.OnReceiveMsg != nil {
		srcPubkey := request.BasicData.Pubkey
		destPubkey := request.DestPubkey
		msgDirection := msg.MsgDirection.From
		responseContent, err := d.OnReceiveMsg(
			srcPubkey,
			destPubkey,
			request.Content,
			request.BasicData.TS,
			request.BasicData.ID,
			msgDirection)
		var retCode *pb.RetCode
		if err != nil {
			retCode = &pb.RetCode{
				Code:   dmsgProtocol.AlreadyExistCode,
				Result: "MsgService->OnPubsubMsgRequest: " + err.Error(),
			}
		}
		return responseContent, retCode, false, nil
	} else {
		log.Warnf("MsgService->OnPubsubMsgRequest: OnReceiveMsg is nil")
	}

	log.Debugf("MsgService->OnPubsubMsgRequest end")
	return nil, nil, false, nil
}

func (d *MsgService) OnPubsubMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf(
		"MsgService->OnPubsubMsgResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Errorf("MsgService->OnPubsubMsgResponse: fail to convert requestProtoData to *pb.MsgReq")
		return nil, fmt.Errorf("MsgService->OnPubsubMsgResponse: fail to convert requestProtoData to *pb.MsgReq")
	}

	response, ok := responseProtoData.(*pb.MsgRes)
	if !ok {
		log.Errorf("MsgService->OnPubsubMsgResponse: fail to convert responseProtoData to *pb.MsgRes")
		return nil, fmt.Errorf("MsgService->OnPubsubMsgResponse: fail to convert responseProtoData to *pb.MsgRes")
	}

	if response.RetCode.Code != 0 {
		log.Warnf("MsgService->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("MsgService->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
	} else {
		if d.OnSendMsgResponse != nil {
			srcPubkey := request.BasicData.Pubkey
			destPubkey := request.DestPubkey
			msgDirection := msg.MsgDirection.From
			d.OnSendMsgResponse(
				srcPubkey,
				destPubkey,
				request.Content,
				request.BasicData.TS,
				request.BasicData.ID,
				msgDirection)
		} else {
			log.Debugf("MsgService->OnPubsubMsgResponse: onSendMsgResponse is nil")
		}
	}
	log.Debugf("MsgService->OnPubsubMsgResponse end")
	return nil, nil
}
