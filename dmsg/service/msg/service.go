package msg

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

var log = ipfsLog.Logger("dmsg.service.msg")

type MsgService struct {
	dmsgServiceCommon.ProxyPubsubService
}

func CreateService(tvbase tvbaseCommon.TvBaseService) (*MsgService, error) {
	d := &MsgService{}
	cfg := tvbase.GetConfig().DMsg
	err := d.Init(tvbase, cfg.MaxMsgCount, cfg.KeepMsgDay)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// sdk-common
func (d *MsgService) Start(
	enableService bool,
	pubkeyData []byte,
	getSig dmsgKey.GetSigCallback,
	timeout time.Duration,
	enableLightUserPubsub bool,
) error {
	log.Debugf("MsgService->Start begin\nenableService: %v", enableService)
	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()
	createPubsubProtocol := adapter.NewCreateMsgPubsubProtocol(ctx, host, d, d, enableService)
	pubsubMsgProtocol := adapter.NewPubsubMsgProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(pubsubMsgProtocol.Adapter.GetRequestPID(), pubsubMsgProtocol)
	d.RegistPubsubProtocol(pubsubMsgProtocol.Adapter.GetResponsePID(), pubsubMsgProtocol)
	err := d.ProxyPubsubService.Start(enableService, pubkeyData, getSig, createPubsubProtocol, pubsubMsgProtocol, enableLightUserPubsub)
	if err != nil {
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
	err := d.SubscribePubsub(pubkey, true, false, false)
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

	msgDirection := msg.MsgDirection.From
	if request.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		msgDirection = msg.MsgDirection.To
		log.Debugf("MsgService->OnPubsubMsgRequest: request.BasicData.PeerID == d.TvBase.GetHost().ID")
	} else if request.DestPubkey != d.LightUser.Key.PubkeyHex {
		log.Errorf(
			"MsgService->OnPubsubMsgRequest: LightUser pubkey isn't equal to destPubkey, d.LightUser.Key.PubkeyHex: %s",
			d.LightUser.Key.PubkeyHex)
		return nil, nil, true,
			fmt.Errorf(
				"MsgService->OnPubsubMsgRequest: LightUser pubkey isn't equal to destPubkey, d.LightUser.Key.PubkeyHex: %s",
				d.LightUser.Key.PubkeyHex)
	}

	if d.OnMsgRequest != nil {
		requestPubkey := request.BasicData.Pubkey
		requestDestPubkey := request.DestPubkey

		responseContent, err := d.OnMsgRequest(
			requestPubkey,
			requestDestPubkey,
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
		log.Debugf("MsgService->OnPubsubMsgResponse: fail to convert requestProtoData to *pb.MsgReq")
		// return nil, fmt.Errorf("MsgService->OnPubsubMsgResponse: fail to convert requestProtoData to *pb.MsgReq")
	}

	response, ok := responseProtoData.(*pb.MsgRes)
	if !ok {
		log.Errorf("MsgService->OnPubsubMsgResponse: fail to convert responseProtoData to *pb.MsgRes")
		return nil, fmt.Errorf("MsgService->OnPubsubMsgResponse: fail to convert responseProtoData to *pb.MsgRes")
	}

	if response.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		log.Debugf("MsgService->OnPubsubMsgResponse: request.BasicData.PeerID == d.TvBase.GetHost().ID()")
	}

	if response.RetCode.Code != 0 {
		log.Warnf("MsgService->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("MsgService->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
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
			log.Debugf("MsgService->OnPubsubMsgResponse: onSendMsgResponse is nil")
		}
	}
	log.Debugf("MsgService->OnPubsubMsgResponse end")
	return nil, nil
}

// DmsgServiceInterface
func (d *MsgService) GetPublishTarget(requestProtoData protoreflect.ProtoMessage) (*dmsgUser.Target, error) {
	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Errorf("MsgService->GetPublishTarget: fail to convert requestProtoData to *pb.MsgReq")
		return nil, fmt.Errorf("MsgService->GetPublishTarget: cannot convert to *pb.MsgReq")
	}

	pubkey := request.DestPubkey
	var target *dmsgUser.Target
	if d.ProxyPubsubList[pubkey] != nil {
		target = &d.ProxyPubsubList[pubkey].Target
	} else if d.LightUser.Key.PubkeyHex == pubkey {
		target = &d.LightUser.Target
	}

	if target == nil {
		log.Errorf("MsgService->GetPublishTarget: target is nil")
		return nil, fmt.Errorf("MsgService->GetPublishTarget: target is nil")
	}
	return target, nil
}
