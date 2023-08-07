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
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/dmsg"
	dmsgClientCommon "github.com/tinyverse-web3/tvbase/dmsg/client/common"
	clientProtocol "github.com/tinyverse-web3/tvbase/dmsg/client/protocol"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	keyUtil "github.com/tinyverse-web3/tvutil/key"
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
	CurSrcUserInfo               *dmsgClientCommon.SrcUserInfo
	onReceiveMsg                 dmsgClientCommon.OnReceiveMsg
	srcUserInfoList              map[string]*dmsgClientCommon.SrcUserInfo
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

	d.srcUserInfoList = make(map[string]*dmsgClientCommon.SrcUserInfo)
	d.destUserInfoList = make(map[string]*dmsgClientCommon.DestUserInfo)
	d.pubChannelInfoList = make(map[string]*dmsgClientCommon.PubChannelInfo)

	d.customStreamProtocolInfoList = make(map[string]*dmsgClientCommon.CustomStreamProtocolInfo)
	d.customPubsubProtocolInfoList = make(map[string]*dmsgClientCommon.CustomPubsubProtocolInfo)
	return nil
}

func (d *DmsgService) getSrcUserInfo(userPubkey string) *dmsgClientCommon.SrcUserInfo {
	return d.srcUserInfoList[userPubkey]
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
	d.UnSubscribeSrcUsers()
	d.UnSubscribeDestUsers()
	return nil
}

func (d *DmsgService) InitUser(
	userPubkeyData []byte,
	getSigCallback dmsgClientCommon.GetSigCallback,
	done chan error,
) error {
	dmsgLog.Logger.Debug("DmsgService->InitUser begin")
	userPubkey := keyUtil.TranslateKeyProtoBufToString(userPubkeyData)
	var err error
	d.CurSrcUserInfo, err = d.SubscribeSrcUser(userPubkey, getSigCallback, true)
	if err != nil {
		return err
	}

	initMailbox := func() {
		dmsgLog.Logger.Debug("DmsgService->InitUser: initMailbox")
		_, err = d.seekMailboxProtocol.Request(d.CurSrcUserInfo.UserKey.PubkeyHex, d.CurSrcUserInfo.UserKey.PubkeyHex)
		if err != nil {
			done <- err
			return
		}
		select {
		case result := <-d.CurSrcUserInfo.MailboxCreateChan:
			if result {
				dmsgLog.Logger.Debug("DmsgService->InitUser: Seek found the mailbox has existed, skip create mailbox")
				done <- nil
				return
			} else {
				// never happen
				done <- fmt.Errorf("DmsgService->InitUser: MailboxCreateChan is false")
				return
			}
		case <-time.After(3 * time.Second):
			dmsgLog.Logger.Debugf("DmsgService->InitUser: time.After 3s, no seek info, will create a new mailbox")
			// begin create mailbox

			hostId := d.BaseService.GetHost().ID().String()
			servicePeerList, err := d.BaseService.GetAvailableServicePeerList(hostId)
			if err != nil {
				dmsgLog.Logger.Errorf("DmsgService->InitUser: getAvailableServicePeerList error: %v", err)
				done <- err
				return
			}

			for _, servicePeerID := range servicePeerList {
				dmsgLog.Logger.Debugf("DmsgService->InitUser: servicePeerID: %v", servicePeerID)
				_, err := d.createMailboxProtocol.Request(servicePeerID, userPubkey)
				if err != nil {
					dmsgLog.Logger.Warnf("DmsgService->InitUser: createMailboxProtocol.Request error: %v", err)
					continue
				}

				select {
				case result := <-d.CurSrcUserInfo.MailboxCreateChan:
					if result {
						done <- nil
						return
					} else {
						continue
					}
				case <-time.After(time.Second * 3):
					continue
				case <-d.BaseService.GetCtx().Done():
					dmsgLog.Logger.Debug("DmsgService->InitUser: BaseService.GetCtx().Done()")
					done <- fmt.Errorf("DmsgService->InitUser: BaseService.GetCtx().Done()")
					return
				}
			}

			dmsgLog.Logger.Error("DmsgService->InitUser: no available mailbox service peers found")
			done <- fmt.Errorf("DmsgService->InitUser: no available mailbox service peers found")
			return
			// end create mailbox
		case <-d.BaseService.GetCtx().Done():
			dmsgLog.Logger.Debug("DmsgService->InitUser: BaseService.GetCtx().Done()")
			done <- fmt.Errorf("DmsgService->InitUser: BaseService.GetCtx().Done()")
			return
		}
	}
	d.BaseService.RegistRendezvousCallback(initMailbox)
	dmsgLog.Logger.Debug("DmsgService->InitUser end")
	return nil
}

func (d *DmsgService) IsExistSrcUser(userPubkey string) bool {
	return d.getSrcUserInfo(userPubkey) != nil
}

func (d *DmsgService) IsExistDestUser(userPubkey string) bool {
	return d.getDestUserInfo(userPubkey) != nil
}

func (d *DmsgService) GetCurSrcUserPubKeyHex() string {
	return d.CurSrcUserInfo.UserKey.PubkeyHex
}

func (d *DmsgService) SubscribeSrcUser(userPubkeyHex string, getSigCallback dmsgClientCommon.GetSigCallback, isReadPubsubMsg bool) (*dmsgClientCommon.SrcUserInfo, error) {
	dmsgLog.Logger.Debugf("DmsgService->SubscribeSrcUser begin: userPubkey: %s", userPubkeyHex)

	if d.IsExistSrcUser(userPubkeyHex) {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: user key(%s) is already exist in srcUserInfoList", userPubkeyHex)
		return nil, fmt.Errorf("DmsgService->SubscribeSrcUser:user key(%s) is already exist in srcUserInfoList", userPubkeyHex)
	}
	if d.IsExistDestUser(userPubkeyHex) {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: user key(%s) is already exist in destUserInfoList", userPubkeyHex)
		return nil, fmt.Errorf("DmsgService->SubscribeSrcUser: user key(%s) is already exist in destUserInfoList", userPubkeyHex)
	}

	srcUserPubkeyData, err := keyUtil.TranslateKeyStringToProtoBuf(userPubkeyHex)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: TranslateKeyStringToProtoBuf error: %v", err)
		return nil, err
	}
	srcUserPubkey, err := keyUtil.ECDSAProtoBufToPublicKey(srcUserPubkeyData)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: Public key is not ECDSA KEY")
		return nil, err
	}

	userTopic, err := d.Pubsub.Join(userPubkeyHex)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: Join error: %v", err)
		return nil, err
	}

	userSub, err := userTopic.Subscribe()
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: Subscribe error: %v", err)
		return nil, err
	}
	// go d.BaseService.DiscoverRendezvousPeers()

	userInfo := &dmsgClientCommon.SrcUserInfo{}
	userInfo.Topic = userTopic
	userInfo.GetSigCallback = getSigCallback
	userInfo.Subscription = userSub
	userInfo.MailboxCreateChan = make(chan bool)
	userInfo.UserKey = &dmsgClientCommon.SrcUserKey{
		Pubkey:    srcUserPubkey,
		PubkeyHex: userPubkeyHex,
	}
	userInfo.IsReadPubsubMsg = isReadPubsubMsg
	d.srcUserInfoList[userPubkeyHex] = userInfo

	if userInfo.IsReadPubsubMsg {
		err = d.StartReadSrcUserPubsubMsg(userInfo.UserKey.PubkeyHex)
		if err != nil {
			return nil, err
		}
	}

	dmsgLog.Logger.Debug("DmsgService->SubscribeSrcUser end.")
	return userInfo, nil
}

func (d *DmsgService) UnSubscribeSrcUser(userPubKey string) error {
	userInfo := d.getSrcUserInfo(userPubKey)
	if userInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->UnSubscribeSrcUser: user key(%s) is not exist in destUserInfoList", userPubKey)
		return fmt.Errorf("DmsgService->UnSubscribeSrcUser: user key(%s) is not exist in destUserInfoList", userPubKey)
	}

	close(userInfo.MailboxCreateChan)
	err := userInfo.Topic.Close()
	if err != nil {
		dmsgLog.Logger.Warnf("DmsgService->unSubscribeSrcUser: Topic.Close error: %v", err)
	}
	userInfo.CancelCtx()
	userInfo.Subscription.Cancel()
	delete(d.srcUserInfoList, userPubKey)

	return nil
}

func (d *DmsgService) UnSubscribeSrcUsers() error {
	for userPubKey := range d.srcUserInfoList {
		d.UnSubscribeSrcUser(userPubKey)
	}
	return nil
}
func (d *DmsgService) StartReadSrcUserPubsubMsg(srcUserPubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->StartReadSrcUserPubsubMsg begin: srcUserPubkey: %v", srcUserPubkey)
	srcUserInfo := d.getSrcUserInfo(srcUserPubkey)
	if srcUserInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->StartReadSrcUserPubsubMsg: src user info(%v) pubsub is not exist", srcUserInfo)
		return fmt.Errorf("dmsgService->StartReadSrcUserPubsubMsg: src user info(%v) pubsub is not exist", srcUserInfo)
	}
	go d.readUserPubsub(&srcUserInfo.UserPubsub)
	dmsgLog.Logger.Debug("DmsgService->StartReadSrcUserPubsubMsg end")
	return nil
}

func (d *DmsgService) StopReadSrcUserPubsubMsg(srcUserPubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->StopReadSrcUserPubsubMsg begin: srcUserPubkey: %v", srcUserPubkey)
	srcUserInfo := d.getSrcUserInfo(srcUserPubkey)
	if srcUserInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->StopReadSrcUserPubsubMsg: src user info(%v) pubsub is not exist", srcUserInfo)
		return fmt.Errorf("dmsgService->StopReadSrcUserPubsubMsg: src user info(%v) pubsub is not exist", srcUserInfo)
	}
	if srcUserInfo.IsReadPubsubMsg {
		srcUserInfo.CancelCtx()
		srcUserInfo.IsReadPubsubMsg = false
	}
	dmsgLog.Logger.Debug("DmsgService->StopReadSrcUserPubsubMsg end")
	return nil
}

func (d *DmsgService) SetReadAllSrcUserPubsubMsg(enable bool) {
	if enable {
		for userPubkey, userInfo := range d.srcUserInfoList {
			if !userInfo.IsReadPubsubMsg {
				userInfo.IsReadPubsubMsg = true
				d.StartReadSrcUserPubsubMsg(userPubkey)
			}
		}
	} else {
		for userPubkey, userInfo := range d.srcUserInfoList {
			if userInfo.IsReadPubsubMsg {
				userInfo.IsReadPubsubMsg = false
				d.StopReadSrcUserPubsubMsg(userPubkey)
			}
		}
	}
}

// dest user
func (d *DmsgService) SubscribeDestUser(userPubkey string, isReadPubsubMsg bool) error {
	dmsgLog.Logger.Debugf("DmsgService->subscribeDestUser begin: userPubkey: %s", userPubkey)

	if d.IsExistDestUser(userPubkey) {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: user key(%s) is already exist in destUserInfoList", userPubkey)
		return fmt.Errorf("DmsgService->SubscribeSrcUser: user key(%s) is already exist in destUserInfoList", userPubkey)
	}
	if d.IsExistSrcUser(userPubkey) {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: user key(%s) is already exist in srcUserInfoList", userPubkey)
		return fmt.Errorf("DmsgService->SubscribeSrcUser: user key(%s) is already exist in srcUserInfoList", userPubkey)
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
	destUserInfo.IsReadPubsubMsg = isReadPubsubMsg
	destUserInfo.Subscription = userSub
	d.destUserInfoList[userPubkey] = destUserInfo
	if destUserInfo.IsReadPubsubMsg {
		err = d.StartReadDestUserPubsubMsg(userPubkey)
		if err != nil {
			return err
		}
	}

	dmsgLog.Logger.Debug("DmsgService->subscribeDestUser: end")
	return nil
}

func (d *DmsgService) UnSubscribeDestUser(userPubKey string) error {
	dmsgLog.Logger.Debugf("DmsgService->unSubscribeDestUser begin: userPubKey: %s", userPubKey)

	userInfo := d.getDestUserInfo(userPubKey)
	if userInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->UnSubscribeDestUser: public key(%s) pubsub is not exist", userPubKey)
		return fmt.Errorf("DmsgService->UnSubscribeDestUser: public key(%s) pubsub is not exist", userPubKey)
	}
	err := userInfo.Topic.Close()
	if err != nil {
		dmsgLog.Logger.Warnf("DmsgService->unSubscribeDestUser: userTopic.Close error: %v", err)
	}

	if userInfo.CancelCtx != nil {
		userInfo.CancelCtx()
	}
	userInfo.Subscription.Cancel()
	delete(d.destUserInfoList, userPubKey)

	dmsgLog.Logger.Debug("DmsgService->unSubscribeDestUser end")
	return nil
}

func (d *DmsgService) UnSubscribeDestUsers() error {
	for userPubKey := range d.destUserInfoList {
		d.UnSubscribeDestUser(userPubKey)
	}
	return nil
}

func (d *DmsgService) StartReadDestUserPubsubMsg(destUserPubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->StartReadDestUserPubsubMsg begin: destUserPubkey: %v", destUserPubkey)
	destUserInfo := d.getDestUserInfo(destUserPubkey)
	if destUserInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeDestUser: user public key(%s) pubsub already exist", destUserPubkey)
		return fmt.Errorf("DmsgService->SubscribeDestUser: user public key(%s) pubsub already exist", destUserPubkey)
	}
	go d.readUserPubsub(&destUserInfo.UserPubsub)
	dmsgLog.Logger.Debug("DmsgService->StartReadDestUserPubsubMsg end")
	return nil
}

func (d *DmsgService) StopReadDestUserPubsubMsg(destUserPubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->StopReadDestUserPubsubMsg begin: srcUserPubkey: %v", destUserPubkey)
	destUserInfo := d.getDestUserInfo(destUserPubkey)
	if destUserInfo == nil {
		return fmt.Errorf("DmsgService->StopDestUserPubsubMsg: cannot find dest user pubsub key %s", destUserPubkey)
	}
	if destUserInfo.IsReadPubsubMsg {
		destUserInfo.CancelCtx()
		destUserInfo.IsReadPubsubMsg = false
	}
	dmsgLog.Logger.Debug("DmsgService->StopReadDestUserPubsubMsg end")
	return nil
}

func (d *DmsgService) SetReadAllDestUserPubsubMsg(enable bool) {
	if enable {
		for destUserPubkey, destUserInfo := range d.destUserInfoList {
			if !destUserInfo.IsReadPubsubMsg {
				destUserInfo.IsReadPubsubMsg = true
				d.StartReadDestUserPubsubMsg(destUserPubkey)
			}
		}
	} else {
		for destUserPubkey, destUserInfo := range d.destUserInfoList {
			if destUserInfo.IsReadPubsubMsg {
				destUserInfo.IsReadPubsubMsg = false
				d.StopReadDestUserPubsubMsg(destUserPubkey)
			}
		}
	}
}

// pub channel
func (d *DmsgService) getPubChannelInfo(userPubkey string) *dmsgClientCommon.PubChannelInfo {
	return d.pubChannelInfoList[userPubkey]
}

func (d *DmsgService) isExistPubChannel(userPubkey string) bool {
	return d.getPubChannelInfo(userPubkey) != nil
}

func (d *DmsgService) startReadPubChannelMsg(pubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->StartPubChannelPubsubMsg begin: destUserPubkey: %v", pubkey)
	pubChannelInfo := d.getPubChannelInfo(pubkey)
	if pubChannelInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->StartPubChannelPubsubMsg: user public key(%s) pubsub already exist", pubkey)
		return fmt.Errorf("DmsgService->StartPubChannelPubsubMsg: user public key(%s) pubsub already exist", pubkey)
	}
	go d.readUserPubsub(&pubChannelInfo.UserPubsub)
	dmsgLog.Logger.Debug("DmsgService->StartPubChannelPubsubMsg end")
	return nil
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
	pubChannelInfo.CreatePubChannelChan = make(chan bool)
	topic, err := d.Pubsub.Join(pubkey)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribePubChannel: Pubsub.Join error: %v", err)
		return err
	}
	subscription, err := topic.Subscribe()
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribePubChannel: Pubsub.Subscribe error: %v", err)
		return err
	}
	pubChannelInfo.Topic = topic
	pubChannelInfo.Subscription = subscription

	err = d.requestCreatePubChannelService(pubChannelInfo)
	if err != nil {
		return err
	}

	d.pubChannelInfoList[pubkey] = pubChannelInfo
	err = d.startReadPubChannelMsg(pubkey)
	if err != nil {
		return err
	}

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

	close(pubChannelInfo.CreatePubChannelChan)

	if pubChannelInfo.CancelCtx != nil {
		pubChannelInfo.CancelCtx()
	}
	pubChannelInfo.Subscription.Cancel()
	delete(d.pubChannelInfoList, userPubKey)

	dmsgLog.Logger.Debug("DmsgService->UnsubscribePubChannel end")
	return nil
}

func (d *DmsgService) requestCreatePubChannelService(pubChannelInfo *dmsgClientCommon.PubChannelInfo) error {
	dmsgLog.Logger.Debugf("DmsgService->requestCreatePubChannelService begin:\npubChannelInfo: %+v", pubChannelInfo)
	find := false

	hostId := d.BaseService.GetHost().ID().String()
	servicePeerList, _ := d.BaseService.GetAvailableServicePeerList(hostId)
	srcPubkey := d.GetCurSrcUserPubKeyHex()
	for _, servicePeerID := range servicePeerList {
		_, err := d.createPubChannelProtocol.Request(servicePeerID, srcPubkey, pubChannelInfo.PubKeyHex)
		if err != nil {
			continue
		}
		find = true
		select {
		case result := <-pubChannelInfo.CreatePubChannelChan:
			if result {
				return nil
			} else {
				continue
			}
		case <-time.After(time.Second * 3):
			continue
		case <-d.BaseService.GetCtx().Done():
			return fmt.Errorf("DmsgService->requestCreatePubChannelService: BaseService.GetCtx().Done()")
		}
	}
	if !find {
		dmsgLog.Logger.Error("DmsgService->requestCreatePubChannelService: no available service peer")
		return fmt.Errorf("DmsgService->requestCreatePubChannelService: no available service peer")
	}
	dmsgLog.Logger.Debug("DmsgService->requestCreatePubChannelService end")
	return nil
}

func (d *DmsgService) GetCurSrcUserSig(protoData []byte) ([]byte, error) {
	srcUserInfo := d.getSrcUserInfo(d.CurSrcUserInfo.UserKey.PubkeyHex)
	if srcUserInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->GetCurSrcUserSig: user public key(%s) pubsub is not exist", d.CurSrcUserInfo.UserKey.PubkeyHex)
		return nil, fmt.Errorf("DmsgService->GetCurSrcUserSig: user public key(%s) pubsub is not exist", d.CurSrcUserInfo.UserKey.PubkeyHex)
	}
	sign, err := srcUserInfo.GetSigCallback(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->GetCurSrcUserSig: %v", err)
		return nil, err
	}
	return sign, nil
}

func (d *DmsgService) SendMsg(destPubkey string, msgContent []byte) (*pb.SendMsgReq, error) {
	dmsgLog.Logger.Debugf("DmsgService->SendMsg begin:\ndestPubkey: %v", destPubkey)
	signPubkey := d.CurSrcUserInfo.UserKey.PubkeyHex
	protoData, err := d.sendMsgPubPrtocol.Request(
		signPubkey,
		destPubkey,
		msgContent,
	)
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

// StreamProtocolCallback interface
func (d *DmsgService) OnCreateMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	// TODO implement me
	_, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxReq", requestProtoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxReq", requestProtoData)
	}
	return nil, nil
}

func (d *DmsgService) OnCreateMailboxResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	request, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxReq", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxReq", responseProtoData)
	}
	response, ok := responseProtoData.(*pb.CreateMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", responseProtoData)
	}

	userInfo := d.getSrcUserInfo(request.BasicData.Pubkey)
	if userInfo == nil {
		dmsgLog.Logger.Warnf("DmsgService->OnCreateMailboxResponse: user public key %s is not exist", response.BasicData.Pubkey)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: user public key %s is not exist", response.BasicData.Pubkey)
	}
	peerId, err := peer.Decode(response.BasicData.PeerID)
	if err != nil {
		return nil, err
	}
	switch response.RetCode.Code {
	case 0: // new
		fallthrough
	case 1: // exist mailbox
		dmsgLog.Logger.Debug("DmsgService->OnCreateMailboxResponse: mailbox has created, read message from mailbox...")
		_, err = d.readMailboxMsgPrtocol.Request(
			peerId,
			response.BasicData.Pubkey)
		if err != nil {
			return nil, err
		}
		if userInfo.MailboxPeerID == "" {
			userInfo.MailboxPeerID = response.BasicData.PeerID
		} else if response.BasicData.PeerID != userInfo.MailboxPeerID {
			_, err = d.releaseMailboxPrtocol.Request(
				peerId,
				response.BasicData.Pubkey)
			if err != nil {
				return nil, err
			}
		}
		userInfo.MailboxCreateChan <- true
	default: // < 0 service no finish create mailbox
		dmsgLog.Logger.Warnf("DmsgService->OnCreateMailboxResponse: RetCode(%v) fail", response.RetCode)
		userInfo.MailboxCreateChan <- false
	}
	return nil, nil
}

func (d *DmsgService) OnCreatePubChannelRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *DmsgService) OnCreatePubChannelResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debugf("dmsgService->OnCreatePubChannelResponse begin:\nrequestProtoData: %v\nresponseProtoData: %v",
		requestProtoData, responseProtoData)

	request, ok := requestProtoData.(*pb.CreatePubChannelReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreatePubChannelResponse: fail to convert request to *pb.CreateMailboxReq")
		return nil, fmt.Errorf("DmsgService->OnCreatePubChannelResponse: fail to convert request to *pb.CreateMailboxReq")
	}
	response, ok := responseProtoData.(*pb.CreatePubChannelRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreatePubChannelResponse: fail to convert response to *pb.CreateMailboxRes")
		return nil, fmt.Errorf("DmsgService->OnCreatePubChannelResponse: fail to convert response to *pb.CreateMailboxRes")
	}

	pubChannelInfo := d.pubChannelInfoList[request.BasicData.Pubkey]
	if pubChannelInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCreatePubChannelResponse: pubkey is not exist")
		return nil, fmt.Errorf("DmsgService->OnCreatePubChannelResponse: pubkey is not exist")
	}

	switch response.RetCode.Code {
	case 0: // new
		fallthrough
	case 1: // exist
		dmsgLog.Logger.Debug("DmsgService->OnCreatePubChannelResponse: create pub channel...")
		pubChannelInfo.CreatePubChannelChan <- true
	default: // < 0
		dmsgLog.Logger.Warnf("DmsgService->OnCreatePubChannelResponse: RetCode(%v) fail", response.RetCode)
		pubChannelInfo.CreatePubChannelChan <- false
	}
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

func (d *DmsgService) OnReadMailboxMsgResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnReadMailboxMsgResponse: begin")

	request, ok := requestProtoData.(*pb.ReadMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.ReleaseMailboxReq", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.ReleaseMailboxReq", responseProtoData)
	}
	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: cannot convert %v to *pb.ReadMailboxMsgRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: cannot convert %v to *pb.ReadMailboxMsgRes", responseProtoData)
	}

	pubkey := request.BasicData.Pubkey
	userInfo := d.getSrcUserInfo(pubkey)
	if userInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: cannot find src user pubuc key %s", pubkey)
		return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: cannot find src user pubuc key %s", pubkey)
	}

	dmsgLog.Logger.Debugf("DmsgService->OnReadMailboxMsgResponse: found (%d) new message...", len(response.ContentList))
	for _, mailboxItem := range response.ContentList {
		dmsgLog.Logger.Debugf("DmsgService->OnReadMailboxMsgResponse: new message Key = %s", mailboxItem.Key)
		msgContent := mailboxItem.Content

		fields := strings.Split(mailboxItem.Key, dmsg.MsgKeyDelimiter)
		if len(fields) < dmsg.MsgFieldsLen {
			dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: msg key fields len not enough:%v", mailboxItem.Key)
			return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: msg key fields len not enough:%v", mailboxItem.Key)
		}

		timeStamp, err := strconv.ParseInt(fields[dmsg.MsgTimeStampIndex], 10, 64)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: msg timeStamp parse err:%v", err)
			return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: msg timeStamp parse err:%v", err)
		}

		destPubkey := fields[dmsg.MsgSrcUserPubKeyIndex]
		srcPubkey := fields[dmsg.MsgDestUserPubKeyIndex]
		msgID := fields[dmsg.MsgIDIndex]

		dmsgLog.Logger.Debugf("DmsgService->OnReadMailboxMsgResponse: From = %s", srcPubkey)
		dmsgLog.Logger.Debugf("DmsgService->OnReadMailboxMsgResponse: To = %s", destPubkey)
		d.onReceiveMsg(
			srcPubkey,
			destPubkey,
			msgContent,
			timeStamp,
			msgID,
			dmsg.MsgDirection.From)

		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: Put user msg error: %v, key:%v", err, mailboxItem.Key)
		}
	}

	dmsgLog.Logger.Debug("DmsgService->OnReadMailboxMsgResponse: end")
	return nil, nil
}

// PubsubProtocolCallback interface
func (d *DmsgService) OnSeekMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *DmsgService) OnSeekMailboxResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	request, ok := requestProtoData.(*pb.SeekMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.ReleaseMailboxReq", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.ReleaseMailboxReq", responseProtoData)
	}
	response, ok := responseProtoData.(*pb.SeekMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnSeekMailboxResponse: cannot convert %v to *pb.SeekMailboxRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnSeekMailboxResponse: cannot convert %v to *pb.SeekMailboxRes", responseProtoData)
	}
	if response.RetCode.Code != 0 {
		dmsgLog.Logger.Warnf("DmsgService->OnSeekMailboxResponse: RetCode(%v) fail", response.RetCode)
		return nil, fmt.Errorf("DmsgService->OnSeekMailboxResponse: RetCode(%v) fail", response.RetCode)
	}

	userPubKey := request.BasicData.Pubkey
	userInfo := d.getSrcUserInfo(userPubKey)
	if userInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnSeekMailboxResponse: cannot find src user pubic key %s", userPubKey)
		return nil, fmt.Errorf("DmsgService->OnSeekMailboxResponse: cannot find src user pubic key %s", userPubKey)
	}
	peerId, err := peer.Decode(response.BasicData.PeerID)
	if err != nil {
		return nil, err
	}
	dmsgLog.Logger.Warnf("DmsgService->OnSeekMailboxResponse: mailbox has existed, read message from mailbox...")
	_, err = d.readMailboxMsgPrtocol.Request(peerId, userPubKey)
	if err != nil {
		return nil, err
	}
	if userInfo.MailboxPeerID == "" {
		userInfo.MailboxPeerID = response.BasicData.PeerID
		userInfo.MailboxCreateChan <- true
		return nil, nil
	} else if response.BasicData.PeerID != userInfo.MailboxPeerID {
		_, err = d.releaseMailboxPrtocol.Request(peerId, userPubKey)
		if err != nil {
			return nil, err
		}
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
	sendMsgReq, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnSendMsgRequest: cannot convert %v to *pb.SendMsgReq", requestProtoData)
		return nil, fmt.Errorf("DmsgService->OnSendMsgRequest: cannot convert %v to *pb.SendMsgReq", requestProtoData)
	}

	srcPubkey := sendMsgReq.BasicData.Pubkey
	destPubkey := sendMsgReq.DestPubkey
	msgDirection := dmsg.MsgDirection.From
	d.onReceiveMsg(
		srcPubkey,
		destPubkey,
		sendMsgReq.Content,
		sendMsgReq.BasicData.TS,
		sendMsgReq.BasicData.ID,
		msgDirection)
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
	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", requestProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", requestProtoData)
	}
	response, ok := responseProtoData.(*pb.CustomProtocolRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", responseProtoData)
	}

	customProtocolInfo := d.customStreamProtocolInfoList[response.PID]
	if customProtocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: customProtocolInfo(%v) is nil", response.PID)
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: customProtocolInfo(%v) is nil", customProtocolInfo)
	}
	if customProtocolInfo.Client == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: Client is nil")
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: Client is nil")
	}

	err := customProtocolInfo.Client.HandleResponse(request, response)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: HandleResponse error: %v", err)
		return nil, err
	}
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
func (d *DmsgService) PublishProtocol(pubkey string, pid pb.PID, protoData []byte) error {
	var userPubsub *dmsgClientCommon.UserPubsub = nil
	destUserInfo := d.getDestUserInfo(pubkey)
	if destUserInfo == nil {
		srcUserInfo := d.getSrcUserInfo(pubkey)
		if srcUserInfo == nil {
			dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: cannot find src/dest user Info for key %s", pubkey)
			return fmt.Errorf("DmsgService->PublishProtocol: cannot find src/dest user Info for key %s", pubkey)
		}
		userPubsub = &srcUserInfo.UserPubsub
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

	err = userPubsub.Topic.Publish(d.BaseService.GetCtx(), pubsubBuf.Bytes())
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: publish protocol error: %v", err)
		return err
	}
	return nil
}

// cmd protocol
func (d *DmsgService) RequestCustomStreamProtocol(peerIdEncode string, pid string, content []byte) error {
	protocolInfo := d.customStreamProtocolInfoList[pid]
	if protocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: protocol %s is not exist", pid)
		return fmt.Errorf("DmsgService->RequestCustomStreamProtocol: protocol %s is not exist", pid)
	}

	peerId, err := peer.Decode(peerIdEncode)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: err: %v", err)
		return err
	}
	_, err = protocolInfo.Protocol.Request(
		peerId,
		d.CurSrcUserInfo.UserKey.PubkeyHex,
		pid,
		content)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: err: %v, servicePeerInfo: %v, user public key: %s, content: %v",
			err, peerId, d.CurSrcUserInfo.UserKey.PubkeyHex, content)
		return err
	}
	return nil
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
