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
	readMailboxMsgPrtocol *dmsgClientCommon.StreamProtocol
	createMailboxProtocol *dmsgClientCommon.StreamProtocol
	releaseMailboxPrtocol *dmsgClientCommon.StreamProtocol
	seekMailboxProtocol   *dmsgClientCommon.PubsubProtocol
	sendMsgPrtocol        *clientProtocol.SendMsgProtocol
}

type DmsgService struct {
	dmsg.DmsgService
	ProtocolProxy
	CurSrcUserInfo               *dmsgClientCommon.SrcUserInfo
	onReceiveMsg                 dmsgClientCommon.OnReceiveMsg
	destUserInfoList             map[string]*dmsgClientCommon.DestUserInfo
	srcUserInfoList              map[string]*dmsgClientCommon.SrcUserInfo
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

	// pubsub protocol
	d.seekMailboxProtocol = clientProtocol.NewSeekMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)
	d.RegPubsubProtocolResCallback(d.seekMailboxProtocol.Adapter.GetResponseProtocolID(), d.seekMailboxProtocol)

	// sendMsgProtocol, special pubsub protocol
	d.sendMsgPrtocol = clientProtocol.NewSendMsgProtocol(d.BaseService.GetHost(), d, d)
	d.RegPubsubProtocolReqCallback(pb.ProtocolID_SEND_MSG_REQ, d.sendMsgPrtocol)

	d.srcUserInfoList = make(map[string]*dmsgClientCommon.SrcUserInfo)
	d.destUserInfoList = make(map[string]*dmsgClientCommon.DestUserInfo)

	d.customStreamProtocolInfoList = make(map[string]*dmsgClientCommon.CustomStreamProtocolInfo)
	d.customPubsubProtocolInfoList = make(map[string]*dmsgClientCommon.CustomPubsubProtocolInfo)
	return nil
}

func (d *DmsgService) initMailbox() error {
	dmsgLog.Logger.Debug("DmsgService->initMailbox begin:")
	var err error
	curUserPubkey := d.CurSrcUserInfo.UserKey.PubkeyHex
	userInfo := d.getSrcUserInfo(curUserPubkey)
	if userInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->initMailbox: current user pubic key %v is not exist", curUserPubkey)
		return fmt.Errorf("DmsgService->initMailbox: current user pubic key %v is not exist", curUserPubkey)
	}

	_, err = d.seekMailboxProtocol.Request(curUserPubkey, curUserPubkey)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->initMailbox: seekMailboxProtocol.Request err: %v", err)
		return err
	}

	select {
	case result := <-userInfo.MailboxCreateSignal:
		if result {
			dmsgLog.Logger.Debug("DmsgService->initMailbox: Seek found the mailbox has existed, skip create mailbox")
			return nil
		}
	case <-time.After(time.Second * 3):
		dmsgLog.Logger.Debug("DmsgService->initMailbox: After 3s, no seek info, will create a new mailbox")
		go d.createMailbox(curUserPubkey)
		return nil
	case <-d.BaseService.GetCtx().Done():
		dmsgLog.Logger.Debug("DmsgService->initMailbox: NodeService.GetCtx().Done()")
		return nil
	}

	dmsgLog.Logger.Debug("DmsgService->initMailbox: end")
	return nil
}

func (d *DmsgService) createMailbox(userPubkey string) {
	dmsgLog.Logger.Debug("DmsgService->createMailbox begin:")
	find := false
	userInfo := d.getSrcUserInfo(userPubkey)
	if userInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->createMailbox: current user pubic key %v is not exist", userPubkey)
		return
	}

	hostId := d.BaseService.GetHost().ID().String()
	servicePeerList, err := d.BaseService.GetAvailableServicePeerList(hostId)
	if err != nil {
		dmsgLog.Logger.Error(err)
	}

	for _, servicePeerID := range servicePeerList {
		err := d.createMailboxProtocol.Request(servicePeerID, userPubkey, userPubkey)
		if err != nil {
			dmsgLog.Logger.Warnf("DmsgService->createMailbox: createMailboxProtocol.Request error: %v", err)
			continue
		}
		find = true
		select {
		case result := <-userInfo.MailboxCreateSignal:
			if result {
				return
			} else {
				continue
			}
		case <-time.After(time.Second * 3):
			continue
		case <-d.BaseService.GetCtx().Done():
			return
		}
	}
	if !find {
		dmsgLog.Logger.Error("DmsgService->createMailbox: no available mailbox service peers found")
		return
	}
	dmsgLog.Logger.Debug("DmsgService->createMailbox: end")
}

func (d *DmsgService) subscribeDestUser(userPubKey string) error {
	dmsgLog.Logger.Debug("DmsgService->subscribeDestUser begin:")
	var err error
	userTopic, err := d.Pubsub.Join(userPubKey)
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

	pubsub := &dmsgClientCommon.DestUserInfo{}
	pubsub.Topic = userTopic
	pubsub.Subscription = userSub
	d.destUserInfoList[userPubKey] = pubsub

	dmsgLog.Logger.Debug("DmsgService->subscribeDestUser: end")
	return nil
}

func (d *DmsgService) unSubscribeDestUser(userPubKey string) error {
	dmsgLog.Logger.Debug("DmsgService->unSubscribeDestUser begin:")
	userInfo := d.destUserInfoList[userPubKey]
	if userInfo != nil {
		err := userInfo.Topic.Close()
		if err != nil {
			dmsgLog.Logger.Warnf("DmsgService->unSubscribeDestUser: userTopic.Close error: %v", err)
		}
		userInfo.CancelFunc()
		userInfo.Subscription.Cancel()

		delete(d.destUserInfoList, userPubKey)
	}
	dmsgLog.Logger.Debug("DmsgService->unSubscribeDestUser end")
	return nil
}

func (d *DmsgService) unSubscribeDestUsers() error {
	for userPubKey := range d.destUserInfoList {
		d.unSubscribeDestUser(userPubKey)
	}
	return nil
}

func (d *DmsgService) unSubscribeSrcUser(userPubKey string) error {
	pubsub := d.srcUserInfoList[userPubKey]
	if pubsub != nil {
		close(pubsub.MailboxCreateSignal)
		err := pubsub.Topic.Close()
		if err != nil {
			dmsgLog.Logger.Warnf("DmsgService->unSubscribeSrcUser: userTopic.Close error: %v", err)
		}
		pubsub.CancelFunc()
		pubsub.Subscription.Cancel()
		delete(d.srcUserInfoList, userPubKey)
	}
	return nil
}

func (d *DmsgService) unSubscribeSrcUsers() error {
	for userPubKey := range d.srcUserInfoList {
		d.unSubscribeSrcUser(userPubKey)
	}
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
	userPubsub.CancelFunc = cancel
	for {
		m, err := userPubsub.Subscription.Next(ctx)
		if err != nil {
			dmsgLog.Logger.Warnf("DmsgService->readUserPubsub: subscription.Next happen err, %v", err)
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
			dmsgLog.Logger.Warnf("DmsgService->readUserPubsub: no find protocolID(%d) for reqSubscribe", protocolID)
		}
		resSubScribe := d.PubsubProtocolResSubscribes[protocolID]
		if resSubScribe != nil {
			resSubScribe.HandleResponseData(contentData)
			continue
		} else {
			dmsgLog.Logger.Warnf("DmsgService->readUserPubsub: no find protocolID(%d) for resSubscribe", protocolID)
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
	d.unSubscribeSrcUsers()
	d.unSubscribeDestUsers()
	return nil
}

func (d *DmsgService) InitUser(userPubkeyData []byte, getSignCallback dmsgClientCommon.GetSignCallback) error {
	dmsgLog.Logger.Debug("DmsgService->InitUser begin:")
	curSrcUserPubkeyHex := keyUtil.TranslateKeyProtoBufToString(userPubkeyData)
	var err error
	d.CurSrcUserInfo, err = d.SubscribeSrcUser(curSrcUserPubkeyHex, getSignCallback, true)
	if err != nil {
		return err
	}

	err = d.initMailbox()
	if err != nil {
		return err
	}
	dmsgLog.Logger.Debug("DmsgService->InitUser end")
	return err
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

func (d *DmsgService) SubscribeSrcUser(srcUserPubkey string, getSignCallback dmsgClientCommon.GetSignCallback, isReadPubsubMsg bool) (*dmsgClientCommon.SrcUserInfo, error) {
	srcUserInfo := d.getSrcUserInfo(srcUserPubkey)
	if srcUserInfo != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: user public key(%s) pubsub already exist", srcUserPubkey)
		return nil, fmt.Errorf("DmsgService->SubscribeSrcUser: user public key(%s) pubsub already exist", srcUserPubkey)
	}

	srcUserPubkeyData, err := keyUtil.TranslateKeyStringToProtoBuf(srcUserPubkey)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: TranslateKeyStringToProtoBuf error: %v", err)
		return nil, err
	}
	srcUserPubkeyObj, err := keyUtil.ECDSAProtoBufToPublicKey(srcUserPubkeyData)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: Public key is not ECDSA KEY")
		return nil, err
	}

	userTopic, err := d.Pubsub.Join(srcUserPubkey)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: error: %v", err)
		return nil, err
	}

	userSub, err := userTopic.Subscribe()
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: SubscribeSrcUser error: %v", err)
		return nil, err
	}
	// go d.BaseService.DiscoverRendezvousPeers()

	srcUserInfo = &dmsgClientCommon.SrcUserInfo{}
	srcUserInfo.Topic = userTopic
	srcUserInfo.GetSignCallback = getSignCallback
	srcUserInfo.Subscription = userSub
	srcUserInfo.MailboxCreateSignal = make(chan bool)
	srcUserInfo.UserKey = &dmsgClientCommon.SrcUserKey{
		Pubkey:    srcUserPubkeyObj,
		PubkeyHex: srcUserPubkey,
	}
	srcUserInfo.IsReadPubsubMsg = isReadPubsubMsg
	d.srcUserInfoList[srcUserPubkey] = srcUserInfo

	if srcUserInfo.IsReadPubsubMsg {
		err = d.StartReadSrcUserPubsubMsg(srcUserInfo.UserKey.PubkeyHex)
		if err != nil {
			return nil, err
		}
	}
	return srcUserInfo, nil
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
		srcUserInfo.CancelFunc()
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
		for userPubkey, userInfo := range d.destUserInfoList {
			if userInfo.IsReadPubsubMsg {
				userInfo.IsReadPubsubMsg = false
				d.StopReadSrcUserPubsubMsg(userPubkey)
			}
		}
	}
}

func (d *DmsgService) GetCurSrcUserSign(protoData []byte) ([]byte, error) {
	srcUserInfo := d.getSrcUserInfo(d.CurSrcUserInfo.UserKey.PubkeyHex)
	if srcUserInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->GetCurSrcUserSign: user public key(%s) pubsub is not exist", d.CurSrcUserInfo.UserKey.PubkeyHex)
		return nil, fmt.Errorf("DmsgService->GetCurSrcUserSign: user public key(%s) pubsub is not exist", d.CurSrcUserInfo.UserKey.PubkeyHex)
	}
	sign, err := srcUserInfo.GetSignCallback(protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->GetCurSrcUserSign: %v", err)
		return nil, err
	}
	return sign, nil
}

func (d *DmsgService) SendMsg(destPubkey string, msgContent []byte) (*pb.SendMsgReq, error) {
	dmsgLog.Logger.Debugf("DmsgService->SendMsg begin: destPubkey: %v", destPubkey)
	data, err := d.sendMsgPrtocol.Request(
		d.CurSrcUserInfo.UserKey.PubkeyHex,
		destPubkey,
		msgContent,
	)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SendMsg: %v", err)
		return nil, err
	}
	sendMsgReq, ok := data.(*pb.SendMsgReq)
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

func (d *DmsgService) SubscribeDestUser(destUserPubkey string, isReadPubsubMsg bool) error {
	destUserInfo := d.getDestUserInfo(destUserPubkey)
	if destUserInfo != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeDestUser: user public key(%s) pubsub already exist", destUserPubkey)
		return fmt.Errorf("DmsgService->SubscribeDestUser: user public key(%s) pubsub already exist", destUserPubkey)
	}
	err := d.subscribeDestUser(destUserPubkey)
	if err != nil {
		return err
	}
	destUserInfo = d.getDestUserInfo(destUserPubkey)

	destUserInfo.IsReadPubsubMsg = isReadPubsubMsg
	if destUserInfo.IsReadPubsubMsg {
		err = d.StartReadDestUserPubsubMsg(destUserPubkey)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DmsgService) UnSubscribeDestUser(destPubkey string) error {
	userInfo := d.getDestUserInfo(destPubkey)
	if userInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->UnSubscribeDestUser: public key(%s) pubsub is not exist", destPubkey)
		return fmt.Errorf("DmsgService->UnSubscribeDestUser: public key(%s) pubsub is not exist", destPubkey)
	}
	err := d.unSubscribeDestUser(destPubkey)
	if err != nil {
		return err
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
		destUserInfo.CancelFunc()
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

// StreamProtocolCallback interface
func (d *DmsgService) OnCreateMailboxResponse(protoData protoreflect.ProtoMessage) (interface{}, error) {
	response, ok := protoData.(*pb.CreateMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", protoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", protoData)
	}
	userInfo := d.getSrcUserInfo(response.BasicData.DestPubkey)
	if userInfo == nil {
		dmsgLog.Logger.Warnf("DmsgService->OnCreateMailboxResponse: user public key %s is not exist", response.BasicData.DestPubkey)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: user public key %s is not exist", response.BasicData.DestPubkey)
	}
	peerId, err := peer.Decode(response.BasicData.PeerId)
	if err != nil {
		return nil, err
	}
	switch response.RetCode.Code {
	case 0: // new
		fallthrough
	case 1: // exist mailbox
		dmsgLog.Logger.Warnf("DmsgService->OnCreateMailboxResponse: mailbox has created, read message from mailbox...")
		err = d.readMailboxMsgPrtocol.Request(
			peerId,
			response.BasicData.DestPubkey,
			response.BasicData.DestPubkey)
		if err != nil {
			return nil, err
		}
		if userInfo.MailboxPeerId == "" {
			userInfo.MailboxPeerId = response.BasicData.PeerId
		} else if response.BasicData.PeerId != userInfo.MailboxPeerId {
			err = d.releaseMailboxPrtocol.Request(
				peerId,
				response.BasicData.DestPubkey,
				response.BasicData.DestPubkey)
			if err != nil {
				return nil, err
			}
		}
		userInfo.MailboxCreateSignal <- true
	default: // < 0 service no finish create mailbox
		dmsgLog.Logger.Warnf("DmsgService->OnCreateMailboxResponse: RetCode(%v) fail", response.RetCode)
		userInfo.MailboxCreateSignal <- false
	}
	return nil, nil
}
func (d *DmsgService) OnReleaseMailboxResponse(protoData protoreflect.ProtoMessage) (interface{}, error) {
	response, ok := protoData.(*pb.ReleaseMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnReleaseMailboxResponse: cannot convert %v to *pb.ReleaseMailboxRes", protoData)
		return nil, fmt.Errorf("DmsgService->OnReleaseMailboxResponse: cannot convert %v to *pb.ReleaseMailboxRes", protoData)
	}
	if response.RetCode.Code != 0 {
		dmsgLog.Logger.Warnf("DmsgService->OnReleaseMailboxResponse: RetCode(%v) fail", response.RetCode)
		return nil, fmt.Errorf("DmsgService->OnReleaseMailboxResponse: RetCode(%v) fail", response.RetCode)
	}
	// nothing to do
	return nil, nil
}

func (d *DmsgService) OnReadMailboxMsgResponse(protoData protoreflect.ProtoMessage) (interface{}, error) {
	dmsgLog.Logger.Debug("DmsgService->OnReadMailboxMsgResponse: new message from mailbox...")
	response, ok := protoData.(*pb.ReadMailboxMsgRes)
	if response == nil || !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: cannot convert %v to *pb.ReadMailboxMsgRes", protoData)
		return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: cannot convert %v to *pb.ReadMailboxMsgRes", protoData)
	}

	pubkey := d.CurSrcUserInfo.UserKey.PubkeyHex
	userInfo := d.getSrcUserInfo(pubkey)
	if userInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: cannot find src user pubuc key %s", pubkey)
		return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: cannot find src user pubuc key %s", pubkey)
	}

	dmsgLog.Logger.Debugf("DmsgService->OnReadMailboxMsgResponse: found (%d) new message...", len(response.MailboxMsgDatas))
	for _, mailboxMsgData := range response.MailboxMsgDatas {

		dmsgLog.Logger.Debugf("DmsgService->OnReadMailboxMsgResponse: new message Key = %s", mailboxMsgData.Key)
		msgContent := mailboxMsgData.MsgContent

		fields := strings.Split(mailboxMsgData.Key, dmsg.MsgKeyDelimiter)
		if len(fields) < dmsg.MsgFieldsLen {
			dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: msg key fields len not enough:%v", mailboxMsgData.Key)
			return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: msg key fields len not enough:%v", mailboxMsgData.Key)
		}

		timeStamp, err := strconv.ParseInt(fields[dmsg.MsgTimeStampIndex], 10, 64)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: msg timeStamp parse err:%v", err)
			return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: msg timeStamp parse err:%v", err)
		}

		destPubkey := fields[dmsg.MsgSrcUserPubKeyIndex]
		srcPubkey := fields[dmsg.MsgDestUserPubKeyIndex]
		msgID := fields[dmsg.MsgIDIndex]

		dmsgLog.Logger.Debug("DmsgService->OnReadMailboxMsgResponse: From = ", srcPubkey)
		dmsgLog.Logger.Debug("DmsgService->OnReadMailboxMsgResponse: To = ", destPubkey)
		d.onReceiveMsg(
			srcPubkey,
			destPubkey,
			msgContent,
			timeStamp,
			msgID,
			dmsg.MsgDirection.From)

		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: Put user msg error: %v, key:%v", err, mailboxMsgData.Key)
		}
	}
	return nil, nil
}

// PubsubProtocolCallback interface
func (d *DmsgService) OnSeekMailboxResponse(protoData protoreflect.ProtoMessage) (interface{}, error) {
	response, ok := protoData.(*pb.SeekMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnSeekMailboxResponse: cannot convert %v to *pb.SeekMailboxRes", protoData)
		return nil, fmt.Errorf("DmsgService->OnSeekMailboxResponse: cannot convert %v to *pb.SeekMailboxRes", protoData)
	}
	if response.RetCode.Code != 0 {
		dmsgLog.Logger.Warnf("DmsgService->OnSeekMailboxResponse: RetCode(%v) fail", response.RetCode)
		return nil, fmt.Errorf("DmsgService->OnSeekMailboxResponse: RetCode(%v) fail", response.RetCode)
	}

	destUserPubKey := response.BasicData.DestPubkey
	userInfo := d.getSrcUserInfo(destUserPubKey)
	if userInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnSeekMailboxResponse: cannot find src user pubic key %s", destUserPubKey)
		return nil, fmt.Errorf("DmsgService->OnSeekMailboxResponse: cannot find src user pubic key %s", destUserPubKey)
	}
	peerId, err := peer.Decode(response.PeerId)
	if err != nil {
		return nil, err
	}
	dmsgLog.Logger.Warnf("DmsgService->OnSeekMailboxResponse: mailbox has existed, read message from mailbox...")
	err = d.readMailboxMsgPrtocol.Request(peerId, destUserPubKey, destUserPubKey)
	if err != nil {
		return nil, err
	}
	if userInfo.MailboxPeerId == "" {
		userInfo.MailboxPeerId = response.BasicData.PeerId
		userInfo.MailboxCreateSignal <- true
		return nil, nil
	} else if response.PeerId != userInfo.MailboxPeerId {
		err = d.releaseMailboxPrtocol.Request(peerId, destUserPubKey, destUserPubKey)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (d *DmsgService) OnCustomStreamProtocolResponse(reqProtoData protoreflect.ProtoMessage, resProtoData protoreflect.ProtoMessage) (interface{}, error) {
	response, ok := resProtoData.(*pb.CustomProtocolRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", resProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", resProtoData)
	}
	customProtocolInfo := d.customStreamProtocolInfoList[response.CustomProtocolID]
	if customProtocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: customProtocolInfo(%v) is nil", response.CustomProtocolID)
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: customProtocolInfo(%v) is nil", customProtocolInfo)
	}
	if customProtocolInfo.Client == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: Client is nil")
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: Client is nil")
	}

	request, ok := reqProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", reqProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomStreamProtocolResponse: cannot convert %v to *pb.CustomContentRes", reqProtoData)
	}

	err := customProtocolInfo.Client.HandleResponse(request, response)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomStreamProtocolResponse: HandleResponse error: %v", err)
		return nil, err
	}
	return nil, nil
}

func (d *DmsgService) OnCustomPubsubProtocolResponse(reqProtoData protoreflect.ProtoMessage, resProtoData protoreflect.ProtoMessage) (interface{}, error) {
	response, ok := resProtoData.(*pb.CustomProtocolRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", resProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", resProtoData)
	}
	customPubsubProtocolInfo := d.customPubsubProtocolInfoList[response.CustomProtocolID]
	if customPubsubProtocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: customProtocolInfo(%v) is nil", response.CustomProtocolID)
		return nil, fmt.Errorf("DmsgService->OnCustomPubsubProtocolResponse: customProtocolInfo(%v) is nil", customPubsubProtocolInfo)
	}
	if customPubsubProtocolInfo.Client == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: Client is nil")
		return nil, fmt.Errorf("DmsgService->OnCustomPubsubProtocolResponse: Client is nil")
	}

	request, ok := reqProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", reqProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomPubsubProtocolResponse: cannot convert %v to *pb.CustomContentRes", reqProtoData)
	}

	err := customPubsubProtocolInfo.Client.HandleResponse(request, response)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomPubsubProtocolResponse: HandleResponse error: %v", err)
		return nil, err
	}
	return nil, nil
}

func (d *DmsgService) OnHandleSendMsgRequest(protoMsg protoreflect.ProtoMessage, protoData []byte) (interface{}, error) {
	sendMsgReq, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnHandleSendMsgRequest: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return nil, fmt.Errorf("DmsgService->OnHandleSendMsgRequest: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}

	srcPubkey := sendMsgReq.SrcPubkey
	destPubkey := sendMsgReq.BasicData.DestPubkey
	msgDirection := dmsg.MsgDirection.From
	d.onReceiveMsg(
		srcPubkey,
		destPubkey,
		sendMsgReq.MsgContent,
		sendMsgReq.BasicData.Timestamp,
		sendMsgReq.BasicData.Id,
		msgDirection)

	return nil, nil
}

// ClientService interface
func (d *DmsgService) PublishProtocol(protocolID pb.ProtocolID, userPubkey string,
	protocolData []byte, pubsubSource dmsgClientCommon.PubsubSourceType) error {
	pubsubBuf := new(bytes.Buffer)
	err := binary.Write(pubsubBuf, binary.LittleEndian, protocolID)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: error: %v", err)
		return err
	}
	err = binary.Write(pubsubBuf, binary.LittleEndian, protocolData)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: error: %v", err)
		return err
	}

	pubsubData := pubsubBuf.Bytes()
	var userPubsub *dmsgClientCommon.UserPubsub = nil
	switch pubsubSource {
	case dmsgClientCommon.PubsubSource.DestUser:
		userInfo := d.getDestUserInfo(userPubkey)
		if userInfo == nil {
			dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: cannot find dest user pubsub key %s", userPubkey)
			return fmt.Errorf("DmsgService->PublishProtocol: cannot find dest user pubsub key %s", userPubkey)
		}
		userPubsub = &userInfo.UserPubsub
	case dmsgClientCommon.PubsubSource.SrcUser:
		userInfo := d.getSrcUserInfo(userPubkey)
		if userInfo == nil {
			dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: cannot find src user pubsub key %s", userPubkey)
			return fmt.Errorf("DmsgService->PublishProtocol: cannot find src user pubsub key %s", userPubkey)
		}
		userPubsub = &userInfo.UserPubsub
	}

	if userPubsub == nil {
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: userPubsub is nil")
		return fmt.Errorf("DmsgService->PublishProtocol: userPubsub is nil")
	}

	err = userPubsub.Topic.Publish(d.BaseService.GetCtx(), pubsubData)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: publish protocol error: %v", err)
		return err
	}
	return nil
}

// cmd protocol
func (d *DmsgService) RequestCustomStreamProtocol(peerId string, customProtocolId string, content []byte) error {
	protocolInfo := d.customStreamProtocolInfoList[customProtocolId]
	if protocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: protocol %s is not exist", customProtocolId)
		return fmt.Errorf("DmsgService->RequestCustomStreamProtocol: protocol %s is not exist", customProtocolId)
	}

	pid, err := peer.Decode(peerId)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: err: %v", err)
		return err
	}
	err = protocolInfo.Protocol.Request(
		pid,
		d.CurSrcUserInfo.UserKey.PubkeyHex,
		d.CurSrcUserInfo.UserKey.PubkeyHex,
		customProtocolId,
		content)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: err: %v, servicePeerInfo: %v, user public key: %s, content: %v",
			err, pid, d.CurSrcUserInfo.UserKey.PubkeyHex, content)
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
