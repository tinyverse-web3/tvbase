package msg

import (
	"fmt"

	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	dmsgCommonService "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MsgService struct {
	dmsgCommonService.LightUserService
	sendMsgProtocol *dmsgProtocol.MsgPProtocol
	onReceiveMsg    msg.OnReceiveMsg
	destUserList    map[string]*dmsgUser.LightUser
}

func CreateService(tvbaseService tvbaseCommon.TvBaseService) (*MsgService, error) {
	d := &MsgService{}
	err := d.Init(tvbaseService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *MsgService) Init(tvbaseService tvbaseCommon.TvBaseService) error {
	err := d.LightUserService.Init(tvbaseService)
	if err != nil {
		return err
	}

	d.destUserList = make(map[string]*dmsgUser.LightUser)
	return nil
}

// sdk-common
func (d *MsgService) Start(
	enableService bool,
	pubkeyData []byte,
	getSig dmsgKey.GetSigCallback) error {
	log.Logger.Debug("MsgService->Start begin")
	err := d.LightUserService.Start(enableService, pubkeyData, getSig, true)
	if err != nil {
		return err
	}

	// pubsub protocol
	d.sendMsgProtocol = adapter.NewSendMsgProtocol(d.TvBase.GetCtx(), d.TvBase.GetHost(), d, d)
	d.RegistPubsubProtocol(d.sendMsgProtocol.Adapter.GetRequestPID(), d.sendMsgProtocol)

	// user
	go d.handlePubsubProtocol(&d.LightUser.Target)

	log.Logger.Debug("MsgService->Start end")
	return nil
}

func (d *MsgService) Stop() error {
	log.Logger.Debug("MsgService->Stop begin")
	err := d.LightUserService.Stop()
	if err != nil {
		return err
	}

	d.UnsubscribeDestUserList()
	log.Logger.Debug("MsgService->Stop end")
	return nil
}

// sdk-destuser
func (d *MsgService) GetDestUser(pubkey string) *dmsgUser.LightUser {
	return d.destUserList[pubkey]
}

func (d *MsgService) SubscribeDestUser(pubkey string) error {
	log.Logger.Debug("MsgService->SubscribeDestUser begin\npubkey: %s", pubkey)
	if d.destUserList[pubkey] != nil {
		log.Logger.Errorf("MsgService->SubscribeDestUser: pubkey is already exist in destUserList")
		return fmt.Errorf("MsgService->SubscribeDestUser: pubkey is already exist in destUserList")
	}
	target, err := dmsgUser.NewTarget(d.TvBase.GetCtx(), pubkey, nil)
	if err != nil {
		log.Logger.Errorf("MsgService->SubscribeDestUser: NewTarget error: %v", err)
		return err
	}
	err = target.InitPubsub(d.Pubsub, pubkey)
	if err != nil {
		log.Logger.Errorf("MsgService->subscribeUser: InitPubsub error: %v", err)
		return err
	}

	user := &dmsgUser.LightUser{
		Target: *target,
	}

	d.destUserList[pubkey] = user
	// go d.BaseService.DiscoverRendezvousPeers()
	return nil
}

func (d *MsgService) UnsubscribeDestUser(pubkey string) error {
	log.Logger.Debugf("MsgService->UnSubscribeDestUser begin\npubkey: %s", pubkey)

	user := d.destUserList[pubkey]
	if user == nil {
		log.Logger.Errorf("MsgService->UnSubscribeDestUser: pubkey is not exist in destUserList")
		return fmt.Errorf("MsgService->UnSubscribeDestUser: pubkey is not exist in destUserList")
	}
	user.Close()
	delete(d.destUserList, pubkey)

	log.Logger.Debug("MsgService->unSubscribeDestUser end")
	return nil
}

func (d *MsgService) UnsubscribeDestUserList() error {
	for userPubKey := range d.destUserList {
		d.UnsubscribeDestUser(userPubKey)
	}
	return nil
}

// sdk-msg
func (d *MsgService) SendMsg(destPubkey string, content []byte) (*pb.SendMsgReq, error) {
	log.Logger.Debugf("MsgService->SendMsg begin:\ndestPubkey: %s", destPubkey)
	requestProtoData, _, err := d.sendMsgProtocol.Request(d.LightUser.Key.PubkeyHex, destPubkey, content)
	if err != nil {
		log.Logger.Errorf("MsgService->SendMsg: sendMsgProtocol.Request error: %v", err)
		return nil, err
	}
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		log.Logger.Errorf("MsgService->SendMsg: requestProtoData is not SendMsgReq")
		return nil, fmt.Errorf("MsgService->SendMsg: requestProtoData is not SendMsgReq")
	}
	log.Logger.Debugf("MsgService->SendMsg end")
	return request, nil
}

func (d *MsgService) SetOnReceiveMsg(onReceiveMsg msg.OnReceiveMsg) {
	d.onReceiveMsg = onReceiveMsg
}

// DmsgServiceInterface
func (d *MsgService) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	var target *dmsgUser.Target = nil
	user := d.destUserList[pubkey]
	if user == nil {
		if d.LightUser.Key.PubkeyHex != pubkey {
			log.Logger.Errorf("MsgService->GetPublishTarget: pubkey not exist")
			return nil, fmt.Errorf("MsgService->GetPublishTarget: pubkey not exist")
		} else {
			target = &d.LightUser.Target
		}
	} else {
		target = &user.Target
	}
	return target, nil
}

// MsgPpCallback
func (d *MsgService) OnSendMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	log.Logger.Debugf("dmsgService->OnSendMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		log.Logger.Errorf("MsgService->OnSendMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
		return nil, nil, fmt.Errorf("MsgService->OnSendMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
	}

	if d.onReceiveMsg != nil {
		srcPubkey := request.BasicData.Pubkey
		destPubkey := request.DestPubkey
		msgDirection := msg.MsgDirection.From
		d.onReceiveMsg(
			srcPubkey,
			destPubkey,
			request.Content,
			request.BasicData.TS,
			request.BasicData.ID,
			msgDirection)
	} else {
		log.Logger.Errorf("MsgService->OnSendMsgRequest: OnReceiveMsg is nil")
	}

	log.Logger.Debugf("dmsgService->OnSendMsgRequest end")
	return nil, nil, nil
}

func (d *MsgService) OnSendMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Logger.Debugf(
		"dmsgService->OnSendMsgResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	response, ok := responseProtoData.(*pb.SendMsgRes)
	if !ok {
		log.Logger.Errorf("MsgService->OnSendMsgResponse: cannot convert %v to *pb.SendMsgRes", responseProtoData)
		return nil, fmt.Errorf("MsgService->OnSendMsgResponse: cannot convert %v to *pb.SendMsgRes", responseProtoData)
	}
	if response.RetCode.Code != 0 {
		log.Logger.Warnf("MsgService->OnSendMsgResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("MsgService->OnSendMsgResponse: fail RetCode: %+v", response.RetCode)
	}
	log.Logger.Debugf("dmsgService->OnSendMsgResponse end")
	return nil, nil
}

// common
func (d *MsgService) handlePubsubProtocol(target *dmsgUser.Target) {
	for {
		m, err := target.WaitMsg()
		if err != nil {
			log.Logger.Warnf("dmsgService->handlePubsubProtocol: target.WaitMsg error: %+v", err)
			return
		}

		log.Logger.Debugf("dmsgService->handlePubsubProtocol:\ntopic: %s\nreceivedFrom: %+v", m.Topic, m.ReceivedFrom)

		protocolID, protocolIDLen, err := d.CheckProtocolData(m.Data)
		if err != nil {
			log.Logger.Errorf("dmsgService->handlePubsubProtocol: CheckPubsubData error: %v", err)
			continue
		}
		protocolData := m.Data[protocolIDLen:]
		protocolHandle := d.ProtocolHandleList[protocolID]
		if protocolHandle != nil {
			err = protocolHandle.HandleRequestData(protocolData)
			if err != nil {
				log.Logger.Warnf("dmsgService->handlePubsubProtocol: HandleRequestData error: %v", err)
			}
			continue
		} else {
			log.Logger.Warnf("dmsgService->handlePubsubProtocol: no protocolHandle for protocolID: %d", protocolID)
		}
	}
}
