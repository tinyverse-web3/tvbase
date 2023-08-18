package msg

import (
	"fmt"

	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"

	ipfsLog "github.com/ipfs/go-log/v2"
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
	dmsgServiceCommon.LightUserService
	pubsubMsgProtocol *dmsgProtocol.PubsubMsgProtocol
	onReceiveMsg      msg.OnReceiveMsg
	destUserList      map[string]*dmsgUser.LightUser
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
	log.Debugf("MsgService->Start begin\nenableService: %v", enableService)

	err := d.LightUserService.Start(enableService, pubkeyData, getSig, true)
	if err != nil {
		return err
	}

	// pubsub protocol
	d.pubsubMsgProtocol = adapter.NewPubsubMsgProtocol(d.TvBase.GetCtx(), d.TvBase.GetHost(), d, d)
	d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetRequestPID(), d.pubsubMsgProtocol)
	d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetResponsePID(), d.pubsubMsgProtocol)

	// user
	d.handlePubsubProtocol(&d.LightUser.Target)

	log.Debug("MsgService->Start end")
	return nil
}

func (d *MsgService) Stop() error {
	log.Debug("MsgService->Stop begin")
	err := d.LightUserService.Stop()
	if err != nil {
		return err
	}

	d.UnsubscribeDestUserList()
	log.Debug("MsgService->Stop end")
	return nil
}

// sdk-destuser
func (d *MsgService) GetDestUser(pubkey string) *dmsgUser.LightUser {
	return d.destUserList[pubkey]
}

func (d *MsgService) SubscribeDestUser(pubkey string) error {
	log.Debug("MsgService->SubscribeDestUser begin\npubkey: %s", pubkey)
	if d.destUserList[pubkey] != nil {
		log.Errorf("MsgService->SubscribeDestUser: pubkey is already exist in destUserList")
		return fmt.Errorf("MsgService->SubscribeDestUser: pubkey is already exist in destUserList")
	}
	target, err := dmsgUser.NewTarget(pubkey, nil)
	if err != nil {
		log.Errorf("MsgService->SubscribeDestUser: NewTarget error: %v", err)
		return err
	}

	err = target.InitPubsub(pubkey)
	if err != nil {
		log.Errorf("MsgService->subscribeUser: InitPubsub error: %v", err)
		return err
	}

	user := &dmsgUser.LightUser{
		Target: *target,
	}

	d.destUserList[pubkey] = user
	return nil
}

func (d *MsgService) UnsubscribeDestUser(pubkey string) error {
	log.Debugf("MsgService->UnSubscribeDestUser begin\npubkey: %s", pubkey)

	user := d.destUserList[pubkey]
	if user == nil {
		log.Errorf("MsgService->UnSubscribeDestUser: pubkey is not exist in destUserList")
		return fmt.Errorf("MsgService->UnSubscribeDestUser: pubkey is not exist in destUserList")
	}
	user.Close()
	delete(d.destUserList, pubkey)

	log.Debug("MsgService->unSubscribeDestUser end")
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
	log.Debugf("MsgService->SendMsg begin:\ndestPubkey: %s", destPubkey)
	requestProtoData, _, err := d.pubsubMsgProtocol.Request(d.LightUser.Key.PubkeyHex, destPubkey, content)
	if err != nil {
		log.Errorf("MsgService->SendMsg: sendMsgProtocol.Request error: %v", err)
		return nil, err
	}
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		log.Errorf("MsgService->SendMsg: requestProtoData is not SendMsgReq")
		return nil, fmt.Errorf("MsgService->SendMsg: requestProtoData is not SendMsgReq")
	}
	log.Debugf("MsgService->SendMsg end")
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
			log.Errorf("MsgService->GetPublishTarget: pubkey not exist")
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
func (d *MsgService) OnPubsubMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("MsgService->OnPubsubMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		log.Errorf("MsgService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
		return nil, nil, false, fmt.Errorf("MsgService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
	}

	if request.DestPubkey != d.LightUser.Key.PubkeyHex {
		log.Errorf("MsgService->OnPubsubMsgRequest: request.DestPubkey != d.LightUser.Key.PubkeyHex")
		return nil, nil, false, fmt.Errorf("MsgService->OnPubsubMsgRequest: request.DestPubkey != d.LightUser.Key.PubkeyHex")
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
		log.Errorf("MsgService->OnPubsubMsgRequest: OnReceiveMsg is nil")
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
	response, ok := responseProtoData.(*pb.SendMsgRes)
	if !ok {
		log.Errorf("MsgService->OnPubsubMsgResponse: cannot convert %v to *pb.SendMsgRes", responseProtoData)
		return nil, fmt.Errorf("MsgService->OnPubsubMsgResponse: cannot convert %v to *pb.SendMsgRes", responseProtoData)
	}
	if response.RetCode.Code != 0 {
		log.Warnf("MsgService->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("MsgService->OnPubsubMsgResponse: fail RetCode: %+v", response.RetCode)
	}
	log.Debugf("MsgService->OnPubsubMsgResponse end")
	return nil, nil
}

// common
func (d *MsgService) handlePubsubProtocol(target *dmsgUser.Target) {
	ctx := d.TvBase.GetCtx()
	protocolHandleChan, err := d.WaitMessage(ctx, target.Key.PubkeyHex)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case protocolHandle, ok := <-protocolHandleChan:
				if !ok {
					return
				}
				pid := protocolHandle.PID
				handle := protocolHandle.Handle
				data := protocolHandle.Data

				requestPID := d.pubsubMsgProtocol.Adapter.GetRequestPID()
				responsePID := d.pubsubMsgProtocol.Adapter.GetResponsePID()

				topicName := target.Pubsub.Topic.String()
				log.Debugf("MailboxService->handlePubsubProtocol: protocolID: %d, topicName: %s", pid, topicName)

				switch pid {
				case requestPID:
					err = handle.HandleRequestData(data)
					if err != nil {
						log.Warnf("MsgService->handlePubsubProtocol: HandleRequestData error: %v", err)
					}
					continue
				case responsePID:
					err = handle.HandleResponseData(data)
					if err != nil {
						log.Warnf("MsgService->handlePubsubProtocol: HandleRequestData error: %v", err)
					}
					continue
				}
			}
		}
	}()
}
