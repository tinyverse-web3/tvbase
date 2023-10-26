package client

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	utilKey "github.com/tinyverse-web3/mtv_go_utils/key"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/dmsg"
	dmsgClientCommon "github.com/tinyverse-web3/tvbase/dmsg/client/common"
	clientProtocol "github.com/tinyverse-web3/tvbase/dmsg/client/protocol"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ProtocolProxy struct {
	readMailboxMsgPrtocol    *dmsgClientCommon.StreamProtocol
	createMailboxProtocol    *dmsgClientCommon.StreamProtocol
	releaseMailboxPrtocol    *dmsgClientCommon.StreamProtocol
	createPubChannelProtocol *dmsgClientCommon.StreamProtocol
	seekMailboxProtocol      *dmsgClientCommon.PubsubProtocol
	queryPeerProtocol        *dmsgClientCommon.PubsubProtocol
	sendMsgPubPrtocol        *dmsgClientCommon.PubsubProtocol
}

type DmsgService struct {
	dmsg.DmsgService
	ProtocolProxy
	SrcUserInfo                  *dmsgClientCommon.SrcUserInfo
	onReceiveMsg                 dmsgClientCommon.OnReceiveMsg
	proxyPubkey                  string
	destUserInfoList             map[string]*dmsgClientCommon.DestUserInfo
	pubChannelInfoList           map[string]*dmsgClientCommon.PubChannelInfo
	customStreamProtocolInfoList map[string]*dmsgClientCommon.CustomStreamProtocolInfo
	customPubsubProtocolInfoList map[string]*dmsgClientCommon.CustomPubsubProtocolInfo
}

func CreateService(nodeService tvCommon.TvBaseService) (*DmsgService, error) {
	d := &DmsgService{}
	err := d.Init(nodeService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DmsgService) Init(nodeService tvCommon.TvBaseService) error {
	err := d.DmsgService.Init(nodeService)
	if err != nil {
		return err
	}

	// stream protocol
	d.createMailboxProtocol = clientProtocol.NewCreateMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.releaseMailboxPrtocol = clientProtocol.NewReleaseMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.readMailboxMsgPrtocol = clientProtocol.NewReadMailboxMsgProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.createPubChannelProtocol = clientProtocol.NewCreatePubChannelProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)

	// pubsub protocol
	d.seekMailboxProtocol = clientProtocol.NewSeekMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.RegPubsubProtocolResCallback(d.seekMailboxProtocol.Adapter.GetResponsePID(), d.seekMailboxProtocol)

	d.queryPeerProtocol = clientProtocol.NewQueryPeerProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.RegPubsubProtocolReqCallback(d.queryPeerProtocol.Adapter.GetRequestPID(), d.queryPeerProtocol)

	d.sendMsgPubPrtocol = clientProtocol.NewSendMsgProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.RegPubsubProtocolReqCallback(d.sendMsgPubPrtocol.Adapter.GetRequestPID(), d.sendMsgPubPrtocol)

	d.destUserInfoList = make(map[string]*dmsgClientCommon.DestUserInfo)
	d.pubChannelInfoList = make(map[string]*dmsgClientCommon.PubChannelInfo)

	d.customStreamProtocolInfoList = make(map[string]*dmsgClientCommon.CustomStreamProtocolInfo)
	d.customPubsubProtocolInfoList = make(map[string]*dmsgClientCommon.CustomPubsubProtocolInfo)
	return nil
}

func (d *DmsgService) getDestUserInfo(userPubkey string) *dmsgClientCommon.DestUserInfo {
	return d.destUserInfoList[userPubkey]
}

func (d *DmsgService) readUserPubsub(userPubsub *dmsgClientCommon.UserPubsub) {
	ctx, cancel := context.WithCancel(d.BaseService.GetCtx())
	userPubsub.Ctx = ctx
	userPubsub.CancelCtx = cancel
	for {
		m, err := userPubsub.Subscription.Next(ctx)
		if err != nil {
			dmsgLog.Logger.Warnf("DmsgService->readUserPubsub: subscription.Next error: %v", err)
			return
		}

		if d.BaseService.GetHost().ID() == m.ReceivedFrom {
			continue
		}

		dmsgLog.Logger.Debugf("DmsgService->readUserPubsub: user pubsub msg, topic:%s, receivedFrom:%v", *m.Topic, m.ReceivedFrom)

		protocolID, protocolIDLen, err := d.CheckPubsubData(m.Data)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->readUserPubsub: CheckPubsubData error: %v", err)
			continue
		}
		contentData := m.Data[protocolIDLen:]
		reqSubscribe := d.PubsubProtocolReqSubscribes[protocolID]
		if reqSubscribe != nil {
			reqSubscribe.HandleRequestData(contentData)
			continue
		} else {
			dmsgLog.Logger.Warnf("DmsgService->readUserPubsub: no find request protocolID(%d) for reqSubscribe", protocolID)
		}
		resSubScribe := d.PubsubProtocolResSubscribes[protocolID]
		if resSubScribe != nil {
			resSubScribe.HandleResponseData(contentData)
			continue
		} else {
			dmsgLog.Logger.Warnf("DmsgService->readUserPubsub: no find response protocolID(%d) for resSubscribe", protocolID)
		}
	}
}

// for sdk
func (d *DmsgService) Start() error {
	err := d.DmsgService.Start()
	if err != nil {
		return err
	}
	return nil
}

func (d *DmsgService) Stop() error {
	err := d.DmsgService.Stop()
	if err != nil {
		return err
	}
	d.UnSubscribeSrcUser()
	d.UnSubscribeDestUsers()
	return nil
}

func (d *DmsgService) InitUser(
	userPubkeyData []byte,
	getSigCallback dmsgClientCommon.GetSigCallback,
	isListenMsg bool,
) (chan error, error) {
	dmsgLog.Logger.Debug("DmsgService->InitUser begin")
	userPubkey := utilKey.TranslateKeyProtoBufToString(userPubkeyData)
	err := d.SubscribeSrcUser(userPubkey, getSigCallback, isListenMsg)
	if err != nil {
		return nil, err
	}

	done := make(chan error)
	go func() {
		if d.BaseService.GetIsRendezvous() {
			_, err := d.CreateMailbox(userPubkey)
			done <- err
		} else {
			c := d.BaseService.RegistRendezvousChan()
			select {
			case <-c:
				d.BaseService.UnregistRendezvousChan(c)
				_, err := d.CreateMailbox(userPubkey)
				done <- err
				return
			case <-d.BaseService.GetCtx().Done():
				dmsgLog.Logger.Debug("DmsgService->InitUser: BaseService.GetCtx().Done()")
				return
			}
		}
	}()
	return done, nil

}

func (d *DmsgService) IsExistMailbox(userPubkey string, duration time.Duration) bool {
	_, seekMailboxDoneChan, err := d.seekMailboxProtocol.Request(d.SrcUserInfo.UserKey.PubkeyHex, userPubkey)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->IsExistMailbox: seekMailboxProtocol.Request error : %+v", err)
		return false
	}

	select {
	case seekMailboxResponseProtoData := <-seekMailboxDoneChan:
		dmsgLog.Logger.Debugf("DmsgService->IsExistMailbox: seekMailboxProtoData: %+v", seekMailboxResponseProtoData)
		response, ok := seekMailboxResponseProtoData.(*pb.SeekMailboxRes)
		if !ok || response == nil {
			dmsgLog.Logger.Errorf("DmsgService->IsExistMailbox: seekMailboxProtoData is not SeekMailboxRes")
			return false
		}
		if response.RetCode.Code < 0 {
			dmsgLog.Logger.Errorf("DmsgService->IsExistMailbox: seekMailboxProtoData fail")
			return false
		} else {
			dmsgLog.Logger.Debugf("DmsgService->IsExistMailbox: seekMailboxProtoData success")
			return true
		}
	case <-time.After(duration):
		dmsgLog.Logger.Debugf("DmsgService->IsExistMailbox: time.After 3s timeout")
		return false

	case <-d.BaseService.GetCtx().Done():
		dmsgLog.Logger.Debug("DmsgService->CreateMailbox: BaseService.GetCtx().Done()")
		return false
	}
}

func (d *DmsgService) CreateMailbox(userPubkey string) (existMailbox bool, err error) {
	dmsgLog.Logger.Debug("DmsgService->CreateMailbox begin")
	_, seekMailboxDoneChan, err := d.seekMailboxProtocol.Request(d.SrcUserInfo.UserKey.PubkeyHex, userPubkey)
	if err != nil {
		return false, err
	}
	select {
	case seekMailboxResponseProtoData := <-seekMailboxDoneChan:
		dmsgLog.Logger.Debugf("DmsgService->CreateMailbox: seekMailboxProtoData: %+v", seekMailboxResponseProtoData)
		response, ok := seekMailboxResponseProtoData.(*pb.SeekMailboxRes)
		if !ok || response == nil {
			dmsgLog.Logger.Errorf("DmsgService->CreateMailbox: seekMailboxProtoData is not SeekMailboxRes")
			return false, fmt.Errorf("DmsgService->CreateMailbox: seekMailboxProtoData is not SeekMailboxRes")
		}
	case <-time.After(3 * time.Second):
		dmsgLog.Logger.Debugf("DmsgService->CreateMailbox: time.After 3s, create new mailbox")
		// create new mailbox
		hostId := d.BaseService.GetHost().ID().String()
		servicePeerList, err := d.BaseService.GetAvailableServicePeerList(hostId)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->CreateMailbox: getAvailableServicePeerList error: %v", err)
			return false, err
		}

		for _, servicePeerID := range servicePeerList {
			dmsgLog.Logger.Debugf("DmsgService->CreateMailbox: servicePeerID: %v", servicePeerID)
			_, createMailboxDoneChan, err := d.createMailboxProtocol.Request(servicePeerID, d.SrcUserInfo.UserKey.PubkeyHex)
			if err != nil {
				dmsgLog.Logger.Errorf("DmsgService->CreateMailbox: createMailboxProtocol.Request error: %v", err)
				continue
			}

			select {
			case createMailboxResponseProtoData := <-createMailboxDoneChan:
				dmsgLog.Logger.Debugf("DmsgService->CreateMailbox: createMailboxResponseProtoData: %+v", createMailboxResponseProtoData)
				response, ok := createMailboxResponseProtoData.(*pb.CreateMailboxRes)
				if !ok || response == nil {
					dmsgLog.Logger.Errorf("DmsgService->CreateMailbox: createMailboxDoneChan is not CreateMailboxRes")
					continue
				}
				switch response.RetCode.Code {
				case 0, 1:
					dmsgLog.Logger.Debugf("DmsgService->CreateMailbox: createMailboxProtocol success")
					return false, nil
				default:
					continue
				}
			case <-time.After(time.Second * 3):
				continue
			case <-d.BaseService.GetCtx().Done():
				dmsgLog.Logger.Debug("DmsgService->CreateMailbox: BaseService.GetCtx().Done()")
				return false, d.BaseService.GetCtx().Err()
			}
		}

		dmsgLog.Logger.Error("DmsgService->CreateMailbox: no available service peers")
		return false, fmt.Errorf("DmsgService->CreateMailbox: no available service peers")
		// end create mailbox
	case <-d.BaseService.GetCtx().Done():
		dmsgLog.Logger.Debug("DmsgService->CreateMailbox: BaseService.GetCtx().Done()")
		return false, fmt.Errorf("DmsgService->CreateMailbox: BaseService.GetCtx().Done()")
	}
	return false, nil
}

func (d *DmsgService) IsExistDestUser(userPubkey string) bool {
	return d.getDestUserInfo(userPubkey) != nil
}

func (d *DmsgService) GetUserPubkeyHex() (string, error) {
	return d.SrcUserInfo.UserKey.PubkeyHex, nil
}

func (d *DmsgService) SetProxyPubkey(pubkey string) error {
	if pubkey == "" {
		return fmt.Errorf("MsgService->SetProxyPubkey: pubkey is empty")
	}

	if d.proxyPubkey != "" {
		return fmt.Errorf("MsgService->SetProxyPubkey: proxyPubkey is not empty")
	}
	err := d.SubscribeDestUser(pubkey, true)
	if err != nil {
		return err
	}
	d.proxyPubkey = pubkey
	return nil
}

func (d *DmsgService) ClearProxyPubkey() error {
	proxyPubkey := d.proxyPubkey
	if proxyPubkey != "" {
		err := d.UnSubscribeDestUser(proxyPubkey)
		if err != nil {
			return err
		}
		d.proxyPubkey = ""
	}
	return nil
}

func (d *DmsgService) GetProxyPubkey() string {
	return d.proxyPubkey
}

func (d *DmsgService) SubscribeSrcUser(
	userPubkeyHex string,
	getSigCallback dmsgClientCommon.GetSigCallback,
	isListenMsg bool,
) error {
	dmsgLog.Logger.Debugf("DmsgService->SubscribeSrcUser begin\nuserPubkey: %s", userPubkeyHex)
	if d.SrcUserInfo != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: SrcUserInfo has initialized")
		return fmt.Errorf("DmsgService->SubscribeSrcUser: SrcUserInfo has initialized")
	}

	if d.IsExistDestUser(userPubkeyHex) {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: user key(%s) is already exist in destUserInfoList", userPubkeyHex)
		return fmt.Errorf("DmsgService->SubscribeSrcUser: user key(%s) is already exist in destUserInfoList", userPubkeyHex)
	}

	srcUserPubkeyData, err := utilKey.TranslateKeyStringToProtoBuf(userPubkeyHex)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: TranslateKeyStringToProtoBuf error: %v", err)
		return err
	}
	userPubkey, err := utilKey.ECDSAProtoBufToPublicKey(srcUserPubkeyData)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: Public key is not ECDSA KEY")
		return err
	}

	topic, err := d.Pubsub.Join(userPubkeyHex)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: Join error: %v", err)
		return err
	}

	subscription, err := topic.Subscribe()
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: Subscribe error: %v", err)
		return err
	}
	// go d.BaseService.DiscoverRendezvousPeers()

	d.SrcUserInfo = &dmsgClientCommon.SrcUserInfo{}
	d.SrcUserInfo.Topic = topic
	d.SrcUserInfo.GetSigCallback = getSigCallback
	d.SrcUserInfo.Subscription = subscription
	d.SrcUserInfo.UserKey = &dmsgClientCommon.SrcUserKey{
		Pubkey:    userPubkey,
		PubkeyHex: userPubkeyHex,
	}

	if isListenMsg {
		err = d.StartReadSrcUserPubsubMsg()
		if err != nil {
			return err
		}
	}
	dmsgLog.Logger.Debugf("DmsgService->SubscribeSrcUser end")
	return nil
}

func (d *DmsgService) UnSubscribeSrcUser() error {
	dmsgLog.Logger.Debugf("DmsgService->UnSubscribeSrcUser begin")
	if d.SrcUserInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->UnSubscribeSrcUser: userPubkey is not exist in destUserInfoList")
		return fmt.Errorf("DmsgService->UnSubscribeSrcUser: userPubkey is not exist in destUserInfoList")
	}
	dmsgLog.Logger.Debugf("DmsgService->UnSubscribeSrcUser:\nsrcUserInfo: %+v", d.SrcUserInfo)

	d.SrcUserInfo.CancelCtx()
	d.SrcUserInfo.Subscription.Cancel()
	err := d.SrcUserInfo.Topic.Close()
	if err != nil {
		dmsgLog.Logger.Warnf("DmsgService->unSubscribeSrcUser: Topic.Close error: %v", err)
	}
	d.SrcUserInfo = nil
	dmsgLog.Logger.Debugf("DmsgService->UnSubscribeSrcUser end")
	return nil
}

func (d *DmsgService) StartReadSrcUserPubsubMsg() error {
	dmsgLog.Logger.Debugf("DmsgService->StartReadSrcUserPubsubMsg begin")

	if d.SrcUserInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->StartReadSrcUserPubsubMsg: user info is nil")
		return fmt.Errorf("dmsgService->StartReadSrcUserPubsubMsg: user info is nil")
	}
	go d.readUserPubsub(&d.SrcUserInfo.UserPubsub)
	dmsgLog.Logger.Debug("DmsgService->StartReadSrcUserPubsubMsg end")
	return nil
}

func (d *DmsgService) StopReadSrcUserPubsubMsg() error {
	dmsgLog.Logger.Debugf("DmsgService->StopReadSrcUserPubsubMsg begin")
	if d.SrcUserInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->StopReadSrcUserPubsubMsg: user info is nil")
		return fmt.Errorf("dmsgService->StopReadSrcUserPubsubMsg: user info is nil")
	}
	d.SrcUserInfo.CancelCtx()
	dmsgLog.Logger.Debug("DmsgService->StopReadSrcUserPubsubMsg end")
	return nil
}

// dest user
func (d *DmsgService) SubscribeDestUser(userPubkey string, isListen bool) error {
	dmsgLog.Logger.Debug("DmsgService->subscribeDestUser begin\nuserPubkey: %s", userPubkey)
	if d.IsExistDestUser(userPubkey) {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: user key is already exist in destUserInfoList")
		return fmt.Errorf("DmsgService->SubscribeSrcUser: user key is already exist in destUserInfoList")
	}

	userTopic, err := d.Pubsub.Join(userPubkey)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeDestUser: Pubsub.Join error: %v", err)
		return err
	}
	userSub, err := userTopic.Subscribe()
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeDestUser: Pubsub.Subscribe error: %v", err)
		return err
	}
	// go d.BaseService.DiscoverRendezvousPeers()

	destUserInfo := &dmsgClientCommon.DestUserInfo{}
	destUserInfo.Topic = userTopic

	destUserInfo.Subscription = userSub
	d.destUserInfoList[userPubkey] = destUserInfo

	if isListen {
		err = d.StartReadDestUserPubsubMsg(userPubkey)
		if err != nil {
			return err
		}

	}

	dmsgLog.Logger.Debug("DmsgService->subscribeDestUser end")
	return nil
}

func (d *DmsgService) UnSubscribeDestUser(userPubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->unSubscribeDestUser begin\nuserPubkey: %s", userPubkey)

	userInfo := d.getDestUserInfo(userPubkey)
	if userInfo == nil {
		return fmt.Errorf("DmsgService->UnSubscribeDestUser: userPubkey is not exist in destUserInfoList")
	}

	if userInfo.CancelCtx != nil {
		userInfo.CancelCtx()
	}
	userInfo.Subscription.Cancel()
	err := userInfo.Topic.Close()
	if err != nil {
		dmsgLog.Logger.Warnf("DmsgService->unSubscribeDestUser: userTopic.Close error: %v", err)
	}
	delete(d.destUserInfoList, userPubkey)

	dmsgLog.Logger.Debug("DmsgService->unSubscribeDestUser end")
	return nil
}

func (d *DmsgService) UnSubscribeDestUsers() error {
	for userPubKey := range d.destUserInfoList {
		d.UnSubscribeDestUser(userPubKey)
	}
	return nil
}

func (d *DmsgService) StartReadDestUserPubsubMsg(userPubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->StartReadDestUserPubsubMsg begin: destUserPubkey: %s", userPubkey)
	destUserInfo := d.getDestUserInfo(userPubkey)
	if destUserInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeDestUser: userPubkey already exist in destUserInfoList")
		return fmt.Errorf("DmsgService->SubscribeDestUser: userPubkey already exist in destUserInfoList")
	}
	go d.readUserPubsub(&destUserInfo.UserPubsub)
	dmsgLog.Logger.Debug("DmsgService->StartReadDestUserPubsubMsg end")
	return nil
}

func (d *DmsgService) StopReadDestUserPubsubMsg(userPubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->StopReadDestUserPubsubMsg begin\nuserPubkey: %v", userPubkey)
	destUserInfo := d.getDestUserInfo(userPubkey)
	if destUserInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->StopReadDestUserPubsubMsg: userPubkey already exist in destUserInfoList")
		return fmt.Errorf("DmsgService->StopDestUserPubsubMsg: userPubkey already exist in destUserInfoList")
	}
	destUserInfo.CancelCtx()

	dmsgLog.Logger.Debug("DmsgService->StopReadDestUserPubsubMsg end")
	return nil
}

// pub channel
func (d *DmsgService) getPubChannelInfo(userPubkey string) *dmsgClientCommon.PubChannelInfo {
	return d.pubChannelInfoList[userPubkey]
}

func (d *DmsgService) isExistPubChannel(userPubkey string) bool {
	return d.getPubChannelInfo(userPubkey) != nil
}

func (d *DmsgService) SubscribePubChannel(pubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->SubscribePubChannel begin:\npubkey: %s", pubkey)

	if d.isExistPubChannel(pubkey) {
		dmsgLog.Logger.Errorf("DmsgService->SubscribePubChannel: pubkey is already exist in pubChannelInfoList")
		return fmt.Errorf("DmsgService->SubscribePubChannel: pubkey is already exist in pubChannelInfoList")
	}

	// go d.BaseService.DiscoverRendezvousPeers()

	pubChannelInfo := &dmsgClientCommon.PubChannelInfo{}
	pubChannelInfo.PubKeyHex = pubkey

	d.pubChannelInfoList[pubkey] = pubChannelInfo

	err := d.createPubChannelService(pubChannelInfo)
	if err != nil {
		delete(d.pubChannelInfoList, pubkey)
		return err
	}

	topic, err := d.Pubsub.Join(pubkey)
	if err != nil {
		delete(d.pubChannelInfoList, pubkey)
		dmsgLog.Logger.Errorf("DmsgService->SubscribePubChannel: Pubsub.Join error: %v", err)
		return err
	}
	subscription, err := topic.Subscribe()
	if err != nil {
		delete(d.pubChannelInfoList, pubkey)
		topic.Close()
		dmsgLog.Logger.Errorf("DmsgService->SubscribePubChannel: Pubsub.Subscribe error: %v", err)
		return err
	}
	pubChannelInfo.Topic = topic
	pubChannelInfo.Subscription = subscription
	go d.readUserPubsub(&pubChannelInfo.UserPubsub)

	dmsgLog.Logger.Debug("DmsgService->SubscribePubChannel end")
	return nil
}

func (d *DmsgService) UnsubscribePubChannel(userPubKey string) error {
	dmsgLog.Logger.Debugf("DmsgService->UnsubscribePubChannel begin: userPubKey: %s", userPubKey)

	pubChannelInfo := d.getPubChannelInfo(userPubKey)
	if pubChannelInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->UnsubscribePubChannel: public key(%s) pubsub is not exist", userPubKey)
		return fmt.Errorf("DmsgService->UnsubscribePubChannel: public key(%s) pubsub is not exist", userPubKey)
	}
	err := pubChannelInfo.Topic.Close()
	if err != nil {
		dmsgLog.Logger.Warnf("DmsgService->UnsubscribePubChannel: userTopic.Close error: %v", err)
	}

	if pubChannelInfo.CancelCtx != nil {
		pubChannelInfo.CancelCtx()
	}
	pubChannelInfo.Subscription.Cancel()
	delete(d.pubChannelInfoList, userPubKey)

	dmsgLog.Logger.Debug("DmsgService->UnsubscribePubChannel end")
	return nil
}

func (d *DmsgService) createPubChannelService(pubChannelInfo *dmsgClientCommon.PubChannelInfo) error {
	dmsgLog.Logger.Debugf("DmsgService->createPubChannelService begin:\npublic channel key: %s", pubChannelInfo.PubKeyHex)
	find := false

	hostId := d.BaseService.GetHost().ID().String()
	servicePeerList, _ := d.BaseService.GetAvailableServicePeerList(hostId)
	srcPubkey, err := d.GetUserPubkeyHex()
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->createPubChannelService: GetUserPubkeyHex error: %v", err)
		return err
	}
	for _, servicePeerID := range servicePeerList {
		dmsgLog.Logger.Debugf("DmsgService->createPubChannelService: servicePeerID: %s", servicePeerID)
		_, createPubChannelDoneChan, err := d.createPubChannelProtocol.Request(servicePeerID, srcPubkey, pubChannelInfo.PubKeyHex)
		if err != nil {
			continue
		}

		select {
		case createPubChannelResponseProtoData := <-createPubChannelDoneChan:
			dmsgLog.Logger.Debugf("DmsgService->createPubChannelService:\ncreatePubChannelResponseProtoData: %+v",
				createPubChannelResponseProtoData)
			response, ok := createPubChannelResponseProtoData.(*pb.CreatePubChannelRes)
			if !ok || response == nil {
				dmsgLog.Logger.Errorf("DmsgService->createPubChannelService: createPubChannelResponseProtoData is not CreatePubChannelRes")
				continue
			}
			if response.RetCode.Code < 0 {
				dmsgLog.Logger.Errorf("DmsgService->createPubChannelService: createPubChannel fail")
				continue
			} else {
				dmsgLog.Logger.Debugf("DmsgService->createPubChannelService: createPubChannel success")
				find = true
				return nil
			}

		case <-time.After(time.Second * 3):
			continue
		case <-d.BaseService.GetCtx().Done():
			return fmt.Errorf("DmsgService->createPubChannelService: BaseService.GetCtx().Done()")
		}
	}
	if !find {
		dmsgLog.Logger.Error("DmsgService->createPubChannelService: no available service peer")
		return fmt.Errorf("DmsgService->createPubChannelService: no available service peer")
	}
	dmsgLog.Logger.Debug("DmsgService->createPubChannelService end")
	return nil
}

func (d *DmsgService) GetUserSig(protoData []byte) ([]byte, error) {
	if d.SrcUserInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->GetUserSig: user public key(%s) pubsub is not exist", d.SrcUserInfo.UserKey.PubkeyHex)
		return nil, fmt.Errorf("DmsgService->GetUserSig: user public key(%s) pubsub is not exist", d.SrcUserInfo.UserKey.PubkeyHex)
	}
	sig, err := d.SrcUserInfo.GetSigCallback(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->GetUserSig: %v", err)
		return nil, err
	}
	return sig, nil
}

func (d *DmsgService) SendMsg(destPubkey string, msgContent []byte) (*pb.SendMsgReq, error) {
	dmsgLog.Logger.Debugf("DmsgService->SendMsg begin:\ndestPubkey: %v", destPubkey)
	reqPubkey := d.SrcUserInfo.UserKey.PubkeyHex
	protoData, _, err := d.sendMsgPubPrtocol.Request(reqPubkey, destPubkey, msgContent)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SendMsg: %v", err)
		return nil, err
	}
	sendMsgReq, ok := protoData.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->SendMsg: data is not SendMsgReq")
		return nil, fmt.Errorf("DmsgService->SendMsg: data is not SendMsgReq")
	}
	dmsgLog.Logger.Debugf("DmsgService->SendMsg end")
	return sendMsgReq, nil
}

func (d *DmsgService) SetOnReceiveMsg(onReceiveMsg dmsgClientCommon.OnReceiveMsg) {
	d.onReceiveMsg = onReceiveMsg
}

func (d *DmsgService) RequestReadMailbox(timeout time.Duration) ([]dmsg.Msg, error) {
	pubkey := d.SrcUserInfo.UserKey.PubkeyHex
	if d.proxyPubkey != "" {
		pubkey = d.proxyPubkey
	}

	_, seekMailboxDoneChan, err := d.seekMailboxProtocol.Request(d.SrcUserInfo.UserKey.PubkeyHex, pubkey)
	if err != nil {
		return nil, err
	}

	select {
	case data := <-seekMailboxDoneChan:
		dmsgLog.Logger.Debugf("DmsgService->RequestReadMailbox: seekMailboxResponseProtoData: %+v", data)
		resp, ok := data.(*pb.SeekMailboxRes)
		if !ok || resp == nil {
			dmsgLog.Logger.Errorf("DmsgService->RequestReadMailbox: data is not SeekMailboxRes")
			return nil, fmt.Errorf("DmsgService->RequestReadMailbox: data is not SeekMailboxRes")
		}
		return d.ReadMailbox(resp.BasicData.PeerID, timeout)
	case <-time.After(timeout):
		dmsgLog.Logger.Debugf("DmsgService->RequestReadMailbox: timeout :%v", timeout)
		return nil, fmt.Errorf("DmsgService->RequestReadMailbox: timeout :%v", timeout)
		// end create mailbox
	case <-d.BaseService.GetCtx().Done():
		dmsgLog.Logger.Debug("DmsgService->RequestReadMailbox: BaseService.GetCtx().Done()")
		return nil, fmt.Errorf("DmsgService->RequestReadMailbox: BaseService.GetCtx().Done()")
	}

}

func (d *DmsgService) ReadMailbox(mailPeerID string, timeout time.Duration) ([]dmsg.Msg, error) {
	var msgList []dmsg.Msg
	peerID, err := peer.Decode(mailPeerID)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->ReadMailbox: peer.Decode error: %v", err)
		return msgList, err
	}
	_, readMailboxDoneChan, err := d.readMailboxMsgPrtocol.Request(peerID, d.SrcUserInfo.UserKey.PubkeyHex)
	if err != nil {
		return msgList, err
	}

	select {
	case responseProtoData := <-readMailboxDoneChan:
		dmsgLog.Logger.Debugf("DmsgService->ReadMailbox: responseProtoData: %+v", responseProtoData)
		response, ok := responseProtoData.(*pb.ReadMailboxRes)
		if !ok || response == nil {
			dmsgLog.Logger.Errorf("DmsgService->ReadMailbox: readMailboxDoneChan is not ReadMailboxRes")
			return msgList, fmt.Errorf("DmsgService->ReadMailbox: readMailboxDoneChan is not ReadMailboxRes")
		}
		dmsgLog.Logger.Debugf("DmsgService->ReadMailbox: readMailboxChanDoneChan success")
		msgList, err = d.parseReadMailboxResponse(response, dmsg.MsgDirection.From)
		if err != nil {
			return msgList, err
		}

		return msgList, nil
	case <-time.After(timeout):
		dmsgLog.Logger.Debugf("DmsgService->ReadMailbox: timeout")
		return msgList, fmt.Errorf("DmsgService->ReadMailbox: timeout")
	case <-d.BaseService.GetCtx().Done():
		dmsgLog.Logger.Debugf("DmsgService->ReadMailbox: BaseService.GetCtx().Done()")
		return msgList, fmt.Errorf("DmsgService->ReadMailbox: BaseService.GetCtx().Done()")
	}
}

func (d *DmsgService) ReleaseMailbox(peerIdHex string, pubkey string) error {
	dmsgLog.Logger.Debug("DmsgService->ReleaseMailbox begin")

	peerID, err := peer.Decode(peerIdHex)
	if err != nil {
		dmsgLog.Logger.Warnf("DmsgService->ReleaseMailbox: fail to decode peer id: %v", err)
		return err
	}

	_, releaseMailboxDoneChan, err := d.releaseMailboxPrtocol.Request(peerID, pubkey)
	if err != nil {
		return err
	}
	select {
	case <-releaseMailboxDoneChan:
		if d.SrcUserInfo.MailboxPeerID == peerIdHex {
			d.SrcUserInfo.MailboxPeerID = ""
		}
		dmsgLog.Logger.Debugf("DmsgService->ReleaseMailbox: releaseMailboxDoneChan success")
	case <-time.After(time.Second * 3):
		return fmt.Errorf("DmsgService->ReleaseMailbox: releaseMailboxDoneChan time out")
	case <-d.BaseService.GetCtx().Done():
		return fmt.Errorf("DmsgService->ReleaseMailbox: BaseService.GetCtx().Done()")
	}

	dmsgLog.Logger.Debugf("DmsgService->ReleaseMailbox end")
	return nil
}

func (d *DmsgService) parseReadMailboxResponse(responseProtoData protoreflect.ProtoMessage, direction string) ([]dmsg.Msg, error) {
	dmsgLog.Logger.Debugf("DmsgService->parseReadMailboxResponse begin:\nresponseProtoData: %v", responseProtoData)
	msgList := []dmsg.Msg{}
	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		dmsgLog.Logger.Errorf("DmsgService->parseReadMailboxResponse: fail to convert to *pb.ReadMailboxMsgRes")
		return msgList, fmt.Errorf("DmsgService->parseReadMailboxResponse: fail to convert to *pb.ReadMailboxMsgRes")
	}

	for _, mailboxItem := range response.ContentList {
		dmsgLog.Logger.Debugf("DmsgService->parseReadMailboxResponse: msg key = %s", mailboxItem.Key)
		msgContent := mailboxItem.Content

		fields := strings.Split(mailboxItem.Key, dmsg.MsgKeyDelimiter)
		if len(fields) < dmsg.MsgFieldsLen {
			dmsgLog.Logger.Errorf("DmsgService->parseReadMailboxResponse: msg key fields len not enough")
			return msgList, fmt.Errorf("DmsgService->parseReadMailboxResponse: msg key fields len not enough")
		}

		timeStamp, err := strconv.ParseInt(fields[dmsg.MsgTimeStampIndex], 10, 64)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->parseReadMailboxResponse: msg timeStamp parse error : %v", err)
			return msgList, fmt.Errorf("DmsgService->parseReadMailboxResponse: msg timeStamp parse error : %v", err)
		}

		destPubkey := fields[dmsg.MsgSrcUserPubKeyIndex]
		srcPubkey := fields[dmsg.MsgDestUserPubKeyIndex]
		msgID := fields[dmsg.MsgIDIndex]

		msgList = append(msgList, dmsg.Msg{
			ID:         msgID,
			SrcPubkey:  srcPubkey,
			DestPubkey: destPubkey,
			Content:    msgContent,
			TimeStamp:  timeStamp,
			Direction:  direction,
		})
	}
	dmsgLog.Logger.Debug("DmsgService->parseReadMailboxResponse end")
	return msgList, nil
}

// StreamProtocolCallback interface
func (d *DmsgService) OnCreateMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	_, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxReq", requestProtoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxReq", requestProtoData)
	}
	return nil, nil
}

func (d *DmsgService) OnCreateMailboxResponse(
	reqProtoData protoreflect.ProtoMessage,
	respProtoData protoreflect.ProtoMessage) (any, error) {
	resp, ok := respProtoData.(*pb.CreateMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", respProtoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", respProtoData)
	}
	if resp.RetCode.Code < 0 {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: error RetCode: %+v", resp.RetCode)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: error RetCode: %+v", resp.RetCode)
	}

	if d.SrcUserInfo.MailboxPeerID != "" {
		dmsgLog.Logger.Debugf("DmsgService->OnCreateMailboxResponse: mailbox already exist")
	}
	d.SrcUserInfo.MailboxPeerID = resp.BasicData.PeerID
	return nil, nil
}

func (d *DmsgService) OnCreatePubChannelRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *DmsgService) OnCreatePubChannelResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnCreatePubChannelResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	dmsgLog.Logger.Debugf("dmsgService->OnCreatePubChannelResponse end")
	return nil, nil
}

func (d *DmsgService) OnReleaseMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *DmsgService) OnReleaseMailboxResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	_, ok := requestProtoData.(*pb.ReleaseMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.ReleaseMailboxReq", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.ReleaseMailboxReq", responseProtoData)
	}

	response, ok := responseProtoData.(*pb.ReleaseMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnReleaseMailboxResponse: cannot convert %v to *pb.ReleaseMailboxRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnReleaseMailboxResponse: cannot convert %v to *pb.ReleaseMailboxRes", responseProtoData)
	}
	if response.RetCode.Code != 0 {
		dmsgLog.Logger.Warnf("DmsgService->OnReleaseMailboxResponse: RetCode(%v) fail", response.RetCode)
		return nil, fmt.Errorf("DmsgService->OnReleaseMailboxResponse: RetCode(%v) fail", response.RetCode)
	}
	// nothing to do
	return nil, nil
}

func (d *DmsgService) OnReadMailboxMsgRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *DmsgService) OnReadMailboxMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnReadMailboxMsgResponse: begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: fail convert to *pb.ReadMailboxMsgRes")
		return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: fail convert to *pb.ReadMailboxMsgRes")
	}

	dmsgLog.Logger.Debugf("DmsgService->OnReadMailboxMsgResponse: found (%d) new message", len(response.ContentList))

	msgList, err := d.parseReadMailboxResponse(responseProtoData, dmsg.MsgDirection.From)
	if err != nil {
		return nil, err
	}
	for _, msg := range msgList {
		dmsgLog.Logger.Debugf("DmsgService->OnReadMailboxMsgResponse: From = %s, To = %s", msg.SrcPubkey, msg.DestPubkey)
		if d.onReceiveMsg != nil {
			d.onReceiveMsg(msg.SrcPubkey, msg.DestPubkey, msg.Content, msg.TimeStamp, msg.ID, msg.Direction)
		} else {
			dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: OnReceiveMsg is nil")
		}
	}

	dmsgLog.Logger.Debug("DmsgService->OnReadMailboxMsgResponse end")
	return nil, nil
}

// PubsubProtocolCallback interface
func (d *DmsgService) OnSeekMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *DmsgService) OnSeekMailboxResponse(
	reqProtoData protoreflect.ProtoMessage,
	respProtoData protoreflect.ProtoMessage) (any, error) {
	resp, ok := respProtoData.(*pb.SeekMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnSeekMailboxResponse: cannot convert %v to *pb.SeekMailboxRes", respProtoData)
		return nil, fmt.Errorf("DmsgService->OnSeekMailboxResponse: cannot convert %v to *pb.SeekMailboxRes", respProtoData)
	}

	if resp.RetCode.Code < 0 {
		dmsgLog.Logger.Errorf("DmsgService->OnSeekMailboxResponse: error RetCode: %+v", resp.RetCode)
		return nil, fmt.Errorf("DmsgService->OnSeekMailboxResponse: error RetCode: %+v", resp.RetCode)
	}
	if d.SrcUserInfo.MailboxPeerID != "" {
		dmsgLog.Logger.Debugf("DmsgService->OnSeekMailboxResponse: mailboxPeerID(%s) already set", d.SrcUserInfo.MailboxPeerID)
	} else {
		d.SrcUserInfo.MailboxPeerID = resp.BasicData.PeerID
	}

	return nil, nil
}

func (d *DmsgService) OnQueryPeerRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	// TODO implement it
	return nil, nil
}

func (d *DmsgService) OnQueryPeerResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	// TODO implement it
	return nil, nil
}

func (d *DmsgService) OnSendMsgRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnSendMsgRequest: cannot convert %v to *pb.SendMsgReq", requestProtoData)
		return nil, fmt.Errorf("DmsgService->OnSendMsgRequest: cannot convert %v to *pb.SendMsgReq", requestProtoData)
	}

	pubkey := request.BasicData.Pubkey
	if request.BasicData.ProxyPubkey != "" {
		pubkey = request.BasicData.ProxyPubkey
	}

	if d.onReceiveMsg != nil {
		srcPubkey := pubkey
		destPubkey := request.DestPubkey
		msgDirection := dmsg.MsgDirection.From
		d.onReceiveMsg(
			srcPubkey,
			destPubkey,
			request.Content,
			request.BasicData.TS,
			request.BasicData.ID,
			msgDirection)
	} else {
		dmsgLog.Logger.Errorf("DmsgService->OnSendMsgRequest: OnReceiveMsg is nil")
	}

	return nil, nil
}

func (d *DmsgService) OnSendMsgResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	response, ok := responseProtoData.(*pb.SendMsgRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnSendMsgResponse: cannot convert %v to *pb.SendMsgRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnSendMsgResponse: cannot convert %v to *pb.SendMsgRes", responseProtoData)
	}
	if response.RetCode.Code != 0 {
		dmsgLog.Logger.Warnf("DmsgService->OnSendMsgResponse: RetCode(%v) fail", response.RetCode)
		return nil, fmt.Errorf("DmsgService->OnSendMsgResponse: RetCode(%v) fail", response.RetCode)
	}
	return nil, nil
}

func (d *DmsgService) OnCustomStreamProtocolRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *DmsgService) OnCustomStreamProtocolResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *DmsgService) OnCustomPubsubProtocolRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *DmsgService) OnCustomPubsubProtocolResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", requestProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", requestProtoData)
	}
	response, ok := responseProtoData.(*pb.CustomProtocolRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", responseProtoData)
	}
	customPubsubProtocolInfo := d.customPubsubProtocolInfoList[response.PID]
	if customPubsubProtocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: customProtocolInfo(%v) is nil", response.PID)
		return nil, fmt.Errorf("DmsgService->OnCustomPubsubProtocolResponse: customProtocolInfo(%v) is nil", customPubsubProtocolInfo)
	}
	if customPubsubProtocolInfo.Client == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: Client is nil")
		return nil, fmt.Errorf("DmsgService->OnCustomPubsubProtocolResponse: Client is nil")
	}

	err := customPubsubProtocolInfo.Client.HandleResponse(request, response)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: HandleResponse error: %v", err)
		return nil, err
	}
	return nil, nil
}

// ClientService interface
func (d *DmsgService) PublishProtocol(ctx context.Context, pubkey string, pid pb.PID, protoData []byte) error {
	var userPubsub *dmsgClientCommon.UserPubsub = nil
	destUserInfo := d.getDestUserInfo(pubkey)
	if destUserInfo == nil {
		if d.SrcUserInfo.UserKey.PubkeyHex != pubkey {
			pubChannelInfo := d.pubChannelInfoList[pubkey]
			if pubChannelInfo == nil {
				dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: cannot find src/dest/pubchannel user Info for key %s", pubkey)
				return fmt.Errorf("DmsgService->PublishProtocol: cannot find src/dest/pubchannel user Info for key %s", pubkey)
			}
			userPubsub = &pubChannelInfo.UserPubsub
		} else {
			userPubsub = &d.SrcUserInfo.UserPubsub
		}
	} else {
		userPubsub = &destUserInfo.UserPubsub
	}

	pubsubBuf := new(bytes.Buffer)
	err := binary.Write(pubsubBuf, binary.LittleEndian, pid)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: error: %v", err)
		return err
	}
	err = binary.Write(pubsubBuf, binary.LittleEndian, protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: error: %v", err)
		return err
	}

	err = userPubsub.Topic.Publish(ctx, pubsubBuf.Bytes())
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: publish protocol error: %v", err)
		return err
	}
	return nil
}

// cmd protocol
func (d *DmsgService) RequestCustomStreamProtocol(
	peerIdEncode string,
	pid string,
	content []byte,
) (*pb.CustomProtocolReq, chan any, error) {
	protocolInfo := d.customStreamProtocolInfoList[pid]
	if protocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: protocol %s is not exist", pid)
		return nil, nil, fmt.Errorf("DmsgService->RequestCustomStreamProtocol: protocol %s is not exist", pid)
	}

	peerId, err := peer.Decode(peerIdEncode)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: err: %v", err)
		return nil, nil, err
	}
	request, responseChan, err := protocolInfo.Protocol.Request(peerId, d.SrcUserInfo.UserKey.PubkeyHex, pid, content)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: err: %v, servicePeerInfo: %v, user public key: %s, content: %v",
			err, peerId, d.SrcUserInfo.UserKey.PubkeyHex, content)
		return nil, nil, err
	}
	return request.(*pb.CustomProtocolReq), responseChan, nil
}

func (d *DmsgService) RegistCustomStreamProtocol(client customProtocol.CustomStreamProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customStreamProtocolInfoList[customProtocolID] != nil {
		dmsgLog.Logger.Errorf("DmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("DmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
	}
	d.customStreamProtocolInfoList[customProtocolID] = &dmsgClientCommon.CustomStreamProtocolInfo{
		Protocol: clientProtocol.NewCustomStreamProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), customProtocolID, d, d),
		Client:   client,
	}
	client.SetCtx(d.BaseService.GetCtx())
	client.SetService(d)
	return nil
}

func (d *DmsgService) UnregistCustomStreamProtocol(client customProtocol.CustomStreamProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customStreamProtocolInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("DmsgService->UnregistCustomStreamProtocol: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customStreamProtocolInfoList[customProtocolID] = nil
	return nil
}

func (d *DmsgService) RegistCustomPubsubProtocol(client customProtocol.CustomPubsubProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customPubsubProtocolInfoList[customProtocolID] != nil {
		dmsgLog.Logger.Errorf("DmsgService->RegistCustomPubsubProtocol: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("DmsgService->RegistCustomPubsubProtocol: protocol %s is already exist", customProtocolID)
	}
	d.customPubsubProtocolInfoList[customProtocolID] = &dmsgClientCommon.CustomPubsubProtocolInfo{
		Protocol: clientProtocol.NewCustomPubsubProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d),
		Client:   client,
	}
	client.SetCtx(d.BaseService.GetCtx())
	client.SetService(d)
	return nil
}

func (d *DmsgService) UnregistCustomPubsubProtocol(client customProtocol.CustomPubsubProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customPubsubProtocolInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("DmsgService->UnregistCustomPubsubProtocol: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customPubsubProtocolInfoList[customProtocolID] = nil
	return nil
}
