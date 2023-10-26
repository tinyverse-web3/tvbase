package msg

import (
	"fmt"
	"time"

	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MsgService struct {
	MsgBase
	pubkey string
	getSig dmsgKey.GetSigCallback
}

func NewService(tvbase define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) (*MsgService, error) {
	d := &MsgService{}
	err := d.Init(tvbase, pubkey, getSig)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// sdk-common
func (d *MsgService) Init(tvbase define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) error {
	log.Debugf("MsgService->Init begin")

	cfg := tvbase.GetConfig().DMsg
	err := d.ProxyPubsubService.Init(tvbase, cfg.MaxMsgCount, cfg.KeepMsgDay)
	if err != nil {
		return err
	}
	d.pubkey = pubkey
	d.getSig = getSig

	log.Debug("MsgService->Init end")
	return nil
}

func (d *MsgService) Start() error {
	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()

	if d.createPubsubProtocol == nil {
		d.createPubsubProtocol = adapter.NewCreateMsgPubsubProtocol(ctx, host, d, d, true, d.pubkey)
	}
	if d.pubsubMsgProtocol == nil {
		d.pubsubMsgProtocol = adapter.NewPubsubMsgProtocol(ctx, host, d, d)
		d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetRequestPID(), d.pubsubMsgProtocol)
		d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetResponsePID(), d.pubsubMsgProtocol)
	}

	if d.createPubsubProtocol != nil && d.pubsubMsgProtocol != nil {
		err := d.ProxyPubsubService.Start(d.pubkey, d.getSig, d.createPubsubProtocol, d.pubsubMsgProtocol, false)
		if err != nil {
			return err
		}
	}

	d.CleanRestPubsub(12 * time.Hour)
	d.enable = true
	return nil
}

func (d *MsgService) Stop() error {
	d.enable = false
	return nil
}

func (d *MsgService) Release() error {
	//TODO
	// d.createPubsubProtocol.Release()
	d.createPubsubProtocol = nil

	d.UnregistPubsubProtocol(d.createPubsubProtocol.Adapter.GetRequestPID())
	d.UnregistPubsubProtocol(d.createPubsubProtocol.Adapter.GetResponsePID())
	d.pubsubMsgProtocol = nil

	err := d.Stop()
	if err != nil {
		return err
	}
	return nil
}

func (d *MsgService) GetDestUser(pubkey string) *dmsgUser.ProxyPubsub {
	return d.GetPubsub(pubkey)
}

func (d *MsgService) IsExistDestUser(pubkey string) bool {
	return d.GetPubsub(pubkey) != nil
}

func (d *MsgService) SubscribeDestUser(pubkey string, isListen bool) error {
	log.Debug("MsgService->SubscribeDestUser begin\npubkey: %s", pubkey)
	err := d.SubscribePubsub(pubkey, true, isListen, false)
	if err != nil {
		return err
	}
	return nil
}

func (d *MsgService) UnSubscribeDestUser(pubkey string) error {
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

	direction := msg.MsgDirection.From
	if request.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		direction = msg.MsgDirection.To
		log.Debugf("MsgService->OnPubsubMsgRequest: request.BasicData.PeerID == d.TvBase.GetHost().ID")
	} else if request.DestPubkey != d.LightUser.Key.PubkeyHex {
		log.Warnf("MsgService->OnPubsubMsgRequest: LightUser pubkey isn't equal to destPubkey, d.LightUser.Key.PubkeyHex: %s",
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
				Result: "MsgService->OnPubsubMsgRequest: OnMsgRequest error: " + err.Error(),
			}
		}
	} else {
		log.Warnf("MsgService->OnPubsubMsgRequest: OnReceiveMsg is nil")
	}
	log.Debugf("MsgService->OnPubsubMsgRequest end")
	return responseContent, retCode, false, nil
}

func (d *MsgService) OnPubsubMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf("MsgService->OnPubsubMsgResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
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
			log.Debugf("MsgService->OnPubsubMsgResponse: OnResponseMsg is nil")
		}
	}
	log.Debugf("MsgService->OnPubsubMsgResponse end")
	return nil, nil
}

// DmsgService
func (d *MsgService) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	var target *dmsgUser.Target
	if d.PubsubList[pubkey] != nil {
		target = &d.PubsubList[pubkey].Target
	} else if d.LightUser.Key.PubkeyHex == pubkey {
		target = &d.LightUser.Target
	}

	if target == nil {
		log.Errorf("MsgService->GetPublishTarget: target is nil")
		return nil, fmt.Errorf("MsgService->GetPublishTarget: target is nil")
	}
	return target, nil
}
