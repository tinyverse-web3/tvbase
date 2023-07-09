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
	CurSrcUserInfo         *dmsgClientCommon.SrcUserInfo
	OnReceiveMsg           dmsgClientCommon.OnReceiveMsg
	destUserInfoList       map[string]*dmsgClientCommon.DestUserInfo
	srcUserInfoList        map[string]*dmsgClientCommon.SrcUserInfo
	customProtocolInfoList map[string]*dmsgClientCommon.CustomProtocolInfo
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
	d.createMailboxProtocol = clientProtocol.NewCreateMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d)
	d.releaseMailboxPrtocol = clientProtocol.NewReleaseMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d)
	d.readMailboxMsgPrtocol = clientProtocol.NewReadMailboxMsgProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d)

	// pubsub protocol
	d.seekMailboxProtocol = clientProtocol.NewSeekMailboxProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), d, d)

	// sendMsgProtocol, special pubsub protocol
	d.sendMsgPrtocol = clientProtocol.NewSendMsgProtocol(d.BaseService.GetHost(), d, d)

	d.srcUserInfoList = make(map[string]*dmsgClientCommon.SrcUserInfo)
	d.destUserInfoList = make(map[string]*dmsgClientCommon.DestUserInfo)

	d.customProtocolInfoList = make(map[string]*dmsgClientCommon.CustomProtocolInfo)
	return nil
}

func (d *DmsgService) initMailbox() error {
	dmsgLog.Logger.Debug("DmsgService->initMailbox: ...")
	var err error
	userPubkey := d.CurSrcUserInfo.UserKey.PubkeyHex
	userInfo := d.getSrcUserInfo(userPubkey)
	if userInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->initMailbox: current user pubic key %v is not exist", userPubkey)
		return err
	}

	err = d.seekMailboxProtocol.Request(userPubkey)
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
		go d.createMailbox(userPubkey)
		return nil
	case <-d.BaseService.GetCtx().Done():
		dmsgLog.Logger.Debug("DmsgService->initMailbox: NodeService.GetCtx().Done()")
		return nil
	}

	dmsgLog.Logger.Debug("DmsgService->initMailbox: done")
	return nil
}

func (d *DmsgService) createMailbox(userPubkey string) {
	dmsgLog.Logger.Debug("DmsgService->createMailbox: ...")
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

	for _, servicePeer := range servicePeerList {
		err := d.createMailboxProtocol.Request(servicePeer, userPubkey)
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
	dmsgLog.Logger.Debug("DmsgService->createMailbox: done.")
}

func (d *DmsgService) subscribeDestUser(userPubKey string) error {
	dmsgLog.Logger.Debug("DmsgService->subscribeDestUser: ...")
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
	go d.BaseService.DiscoverRendezvousPeers()

	pubsub := &dmsgClientCommon.DestUserInfo{}
	pubsub.UserTopic = userTopic
	pubsub.UserSub = userSub
	d.destUserInfoList[userPubKey] = pubsub

	dmsgLog.Logger.Debug("DmsgService->subscribeDestUser: done.")
	return nil
}

func (d *DmsgService) unSubscribeDestUser(userPubKey string) error {
	dmsgLog.Logger.Debug("DmsgService->unSubscribeDestUser ...")
	userInfo := d.destUserInfoList[userPubKey]
	if userInfo != nil {
		err := userInfo.UserTopic.Close()
		if err != nil {
			dmsgLog.Logger.Warnln(err)
		}
		userInfo.UserSub.Cancel()
		delete(d.destUserInfoList, userPubKey)
	}
	dmsgLog.Logger.Debug("DmsgService->unSubscribeDestUser done")
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
		err := pubsub.UserTopic.Close()
		if err != nil {
			dmsgLog.Logger.Warnln(err)
		}
		pubsub.UserSub.Cancel()
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
		m, err := userPubsub.UserSub.Next(ctx)
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
			reqSubscribe.OnRequest(m, contentData)
			continue
		} else {
			dmsgLog.Logger.Warnf("DmsgService->readUserPubsub: no find protocolID(%d) for reqSubscribe", protocolID)
		}
		resSubScribe := d.PubsubProtocolResSubscribes[protocolID]
		if resSubScribe != nil {
			resSubScribe.OnResponse(m, contentData)
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

func (d *DmsgService) InitUser(userPubkeyData []byte) error {
	dmsgLog.Logger.Debug("DmsgService->InitUser...")
	curSrcUserPubkeyHex := keyUtil.TranslateKeyProtoBufToString(userPubkeyData)
	var err error
	d.CurSrcUserInfo, err = d.SubscribeSrcUser(curSrcUserPubkeyHex, true)
	if err != nil {
		return err
	}

	err = d.initMailbox()
	if err != nil {
		return err
	}
	dmsgLog.Logger.Debug("DmsgService->InitUser done.")
	return err
}

func (d *DmsgService) IsExistSrcUser(userPubkey string) bool {
	return d.getSrcUserInfo(userPubkey) != nil
}

func (d *DmsgService) IsExistDestUser(userPubkey string) bool {
	return d.getDestUserInfo(userPubkey) != nil
}

func (d *DmsgService) GetCurUserKey() string {
	// return d.CurSrcUserKey.PrikeyHex, d.CurSrcUserKey.PubkeyHex
	return d.CurSrcUserInfo.UserKey.PubkeyHex
}

func (d *DmsgService) SubscribeSrcUser(srcUserPubkey string, isReadPubsubMsg bool) (*dmsgClientCommon.SrcUserInfo, error) {
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
	go d.BaseService.DiscoverRendezvousPeers()

	srcUserInfo = &dmsgClientCommon.SrcUserInfo{}
	srcUserInfo.UserTopic = userTopic
	srcUserInfo.UserSub = userSub
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
	dmsgLog.Logger.Debugf("DmsgService->StartReadSrcUserPubsubMsg, srcUserPubkey: %v", srcUserPubkey)
	srcUserInfo := d.getSrcUserInfo(srcUserPubkey)
	if srcUserInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->StartReadSrcUserPubsubMsg: src user info(%v) pubsub is not exist", srcUserInfo)
		return fmt.Errorf("dmsgService->StartReadSrcUserPubsubMsg: src user info(%v) pubsub is not exist", srcUserInfo)
	}
	go d.readUserPubsub(&srcUserInfo.UserPubsub)
	dmsgLog.Logger.Debug("DmsgService->StartReadSrcUserPubsubMsg done.")
	return nil
}

func (d *DmsgService) StopReadSrcUserPubsubMsg(srcUserPubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->StopReadSrcUserPubsubMsg, srcUserPubkey: %v", srcUserPubkey)
	srcUserInfo := d.getSrcUserInfo(srcUserPubkey)
	if srcUserInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->StopReadSrcUserPubsubMsg: src user info(%v) pubsub is not exist", srcUserInfo)
		return fmt.Errorf("dmsgService->StopReadSrcUserPubsubMsg: src user info(%v) pubsub is not exist", srcUserInfo)
	}
	if srcUserInfo.IsReadPubsubMsg {
		srcUserInfo.CancelFunc()
		srcUserInfo.IsReadPubsubMsg = false
	}
	dmsgLog.Logger.Debug("DmsgService->StopReadSrcUserPubsubMsg done.")
	return nil
}

func (d *DmsgService) SetReadAllSrcUserPubsubMsg(enable bool) {
	if enable {
		for destUserPubkey, srcUserInfo := range d.srcUserInfoList {
			if !srcUserInfo.IsReadPubsubMsg {
				srcUserInfo.IsReadPubsubMsg = true
				d.StartReadSrcUserPubsubMsg(destUserPubkey)
			}
		}
	} else {
		for destUserPubkey, srcUserInfo := range d.srcUserInfoList {
			if srcUserInfo.IsReadPubsubMsg {
				srcUserInfo.IsReadPubsubMsg = false
				d.StopReadSrcUserPubsubMsg(destUserPubkey)
			}
		}
	}
}

func (d *DmsgService) SendMsg(destPubkey string, msgContent []byte, getSigCallback dmsgClientCommon.GetSigCallback) (*pb.SendMsgReq, error) {
	dmsgLog.Logger.Debugf("DmsgService->SendMsg: %v", destPubkey)
	sendMsgData := &dmsg.SendMsgData{
		SrcUserPubkeyHex:  d.CurSrcUserInfo.UserKey.PubkeyHex,
		DestUserPubkeyHex: destPubkey,
		SrcUserPubkey:     d.CurSrcUserInfo.UserKey.Pubkey,
		MsgContent:        msgContent,
	}

	sendMsgReq, err := d.sendMsgPrtocol.Request(sendMsgData, getSigCallback)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SendMsg: %v", err)
		return nil, err
	}
	dmsgLog.Logger.Debugf("DmsgService->SendMsg done.")
	return sendMsgReq, nil
}

func (d *DmsgService) SetOnReceiveMsg(onReceiveMsg dmsgClientCommon.OnReceiveMsg) {
	d.OnReceiveMsg = onReceiveMsg
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
	dmsgLog.Logger.Debugf("DmsgService->StartReadDestUserPubsubMsg: %v", destUserPubkey)
	destUserInfo := d.getDestUserInfo(destUserPubkey)
	if destUserInfo != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeDestUser: user public key(%s) pubsub already exist", destUserPubkey)
		return fmt.Errorf("DmsgService->SubscribeDestUser: user public key(%s) pubsub already exist", destUserPubkey)
	}
	go d.readUserPubsub(&destUserInfo.UserPubsub)
	dmsgLog.Logger.Debug("DmsgService->StartReadDestUserPubsubMsg done.")
	return nil
}

func (d *DmsgService) StopReadDestUserPubsubMsg(destUserPubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->StopReadDestUserPubsubMsg, srcUserPubkey: %v", destUserPubkey)
	destUserInfo := d.getDestUserInfo(destUserPubkey)
	if destUserInfo == nil {
		return fmt.Errorf("DmsgService->StopDestUserPubsubMsg: cannot find dest user pubsub key %s", destUserPubkey)
	}
	if destUserInfo.IsReadPubsubMsg {
		destUserInfo.CancelFunc()
		destUserInfo.IsReadPubsubMsg = false
	}
	dmsgLog.Logger.Debug("DmsgService->StopReadDestUserPubsubMsg done.")
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
		err = d.readMailboxMsgPrtocol.Request(peerId, response.BasicData.DestPubkey)
		if err != nil {
			return nil, err
		}
		if userInfo.MailboxPeerId == "" {
			userInfo.MailboxPeerId = response.BasicData.PeerId
		} else if response.BasicData.PeerId != userInfo.MailboxPeerId {
			err = d.releaseMailboxPrtocol.Request(peerId, response.BasicData.DestPubkey)
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
		d.OnReceiveMsg(
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

	pubkey := response.BasicData.DestPubkey
	userInfo := d.getSrcUserInfo(pubkey)
	if userInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnSeekMailboxResponse: cannot find src user pubic key %s", pubkey)
		return nil, fmt.Errorf("DmsgService->OnSeekMailboxResponse: cannot find src user pubic key %s", pubkey)
	}
	peerId, err := peer.Decode(response.PeerId)
	if err != nil {
		return nil, err
	}
	dmsgLog.Logger.Warnf("DmsgService->OnSeekMailboxResponse: mailbox has existed, read message from mailbox...")
	err = d.readMailboxMsgPrtocol.Request(peerId, pubkey)
	if err != nil {
		return nil, err
	}
	if userInfo.MailboxPeerId == "" {
		userInfo.MailboxPeerId = response.BasicData.PeerId
		userInfo.MailboxCreateSignal <- true
		return nil, nil
	} else if response.PeerId != userInfo.MailboxPeerId {
		err = d.releaseMailboxPrtocol.Request(peerId, pubkey)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (d *DmsgService) OnCustomProtocolResponse(reqProtoData protoreflect.ProtoMessage, resProtoData protoreflect.ProtoMessage) (interface{}, error) {
	response, ok := resProtoData.(*pb.CustomProtocolRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomProtocolResponse: cannot convert %v to *pb.CustomContentRes", resProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomProtocolResponse: cannot convert %v to *pb.CustomContentRes", resProtoData)
	}
	customProtocolInfo := d.customProtocolInfoList[response.CustomProtocolID]
	if customProtocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomProtocolResponse: customProtocolInfo(%v) is nil", response.CustomProtocolID)
		return nil, fmt.Errorf("DmsgService->OnCustomContentRequest: customProtocolInfo(%v) is nil", customProtocolInfo)
	}
	if customProtocolInfo.Client == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomProtocolResponse: callback is nil")
		return nil, fmt.Errorf("DmsgService->OnCustomContentRequest: callback is nil")
	}

	request, ok := reqProtoData.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomProtocolResponse: cannot convert %v to *pb.CustomContentRes", reqProtoData)
		return nil, fmt.Errorf("DmsgService->OnCustomProtocolResponse: cannot convert %v to *pb.CustomContentRes", reqProtoData)
	}

	err := customProtocolInfo.Client.HandleResponse(request, response)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->OnCustomProtocolResponse: OnClientResponse error: %v", err)
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

	// TODO, no need to check src and dest, delete this code after test
	isExistSrc := d.IsExistSrcUser(sendMsgReq.BasicData.DestPubkey)
	isExistDest := d.IsExistDestUser(sendMsgReq.BasicData.DestPubkey)
	if !isExistSrc && !isExistDest {
		dmsgLog.Logger.Errorf("DmsgService->OnHandleSendMsgRequest: cannot find src user public key %v", sendMsgReq.BasicData.DestPubkey)
		return nil, fmt.Errorf("DmsgService->OnHandleSendMsgRequest: cannot find src user public key %v", sendMsgReq.BasicData.DestPubkey)
	}

	srcPubkey := sendMsgReq.SrcPubkey
	destPubkey := sendMsgReq.BasicData.DestPubkey
	msgDirection := dmsg.MsgDirection.From
	d.OnReceiveMsg(
		srcPubkey,
		destPubkey,
		sendMsgReq.MsgContent,
		sendMsgReq.BasicData.Timestamp,
		sendMsgReq.BasicData.Id,
		msgDirection)

	return nil, nil
}

func (d *DmsgService) OnSendMsgBeforePublish(protoMsg protoreflect.ProtoMessage) error {
	sendMsgReq, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->HandleMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return fmt.Errorf("DmsgService->HandleMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}

	if !d.IsExistSrcUser(sendMsgReq.SrcPubkey) {
		return fmt.Errorf("DmsgService->OnSendMsgBeforePublish: not find src user public key %v", sendMsgReq.SrcPubkey)
	}
	srcPubkey := sendMsgReq.SrcPubkey
	destPubkey := sendMsgReq.BasicData.DestPubkey
	if d.OnReceiveMsg != nil {
		d.OnReceiveMsg(
			srcPubkey,
			destPubkey,
			sendMsgReq.MsgContent,
			sendMsgReq.BasicData.Timestamp,
			sendMsgReq.BasicData.Id,
			dmsg.MsgDirection.To)
	} else {
		dmsgLog.Logger.Debugf("DmsgService->OnSendMsgBeforePublish: %v send msg to %v, msgContent:%v", srcPubkey, destPubkey, sendMsgReq.MsgContent)
	}
	return nil
}

// ClientService interface

func (d *DmsgService) PublishProtocol(protocolID pb.ProtocolID, userPubkey string,
	protocolData []byte, pubsubSource dmsgClientCommon.PubsubSourceType) error {
	pubsubBuf := new(bytes.Buffer)
	err := binary.Write(pubsubBuf, binary.LittleEndian, protocolID)
	if err != nil {
		return err
	}
	err = binary.Write(pubsubBuf, binary.LittleEndian, protocolData)
	if err != nil {
		return err
	}

	pubsubData := pubsubBuf.Bytes()
	switch pubsubSource {
	case dmsgClientCommon.PubsubSource.DestUser:
		userInfo := d.getDestUserInfo(userPubkey)
		if userInfo == nil {
			dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: cannot find dest user pubsub key %s", userPubkey)
			return fmt.Errorf("DmsgService->PublishProtocol: cannot find dest user pubsub key %s", userPubkey)
		}
		err := userInfo.UserTopic.Publish(d.BaseService.GetCtx(), pubsubData)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: publish protocol error: %v", err)
			return fmt.Errorf("DmsgService->PublishProtocol: publish protocol error: %v", err)
		}
	case dmsgClientCommon.PubsubSource.SrcUser:
		userInfo := d.getSrcUserInfo(userPubkey)
		if userInfo == nil {
			dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: cannot find src user pubsub key %s", userPubkey)
			return fmt.Errorf("DmsgService->PublishProtocol: cannot find src user pubsub key %s", userPubkey)
		}
		err := userInfo.UserTopic.Publish(d.BaseService.GetCtx(), pubsubData)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: publish protocol error: %v", err)
			return fmt.Errorf("DmsgService->PublishProtocol: publish protocol error: %v", err)
		}
	default:
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: no find userPubkey %s for pubsubSource %v", userPubkey, pubsubSource)
		return fmt.Errorf("DmsgService->PublishProtocol: no find userPubkey %s for pubsubSource %v", userPubkey, pubsubSource)
	}
	return nil
}

// cmd protocol
func (d *DmsgService) RequestCustomProtocol(customProtocolId string, content []byte) error {
	find := false
	protocolInfo := d.customProtocolInfoList[customProtocolId]
	if protocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomProtocol: protocol %s is not exist", customProtocolId)
		return fmt.Errorf("DmsgService->RequestCustomProtocol: protocol %s is not exist", customProtocolId)
	}

	hostId := d.BaseService.GetHost().ID().String()
	servicePeerList, err := d.BaseService.GetAvailableServicePeerList(hostId)
	if err != nil {
		dmsgLog.Logger.Error(err)
	}

	for _, peerInfo := range servicePeerList {
		err := protocolInfo.StreamProtocol.RequestCustomProtocol(peerInfo, d.CurSrcUserInfo.UserKey.PubkeyHex, customProtocolId, content)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->RequestCustomProtocol: err: %v, servicePeerInfo: %v, user public key: %s, content: %v",
				err, peerInfo, d.CurSrcUserInfo.UserKey.PubkeyHex, content)
			// TODO: need broadcast?
			continue
		}
		find = true
		break
	}
	if !find {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomProtocol: no available service peers found")
		return fmt.Errorf("DmsgService->RequestCustomProtocol: no available service peers found")
	}
	return nil
}

func (d *DmsgService) RegistCustomStreamProtocol(client customProtocol.CustomProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customProtocolInfoList[customProtocolID] != nil {
		dmsgLog.Logger.Errorf("DmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
		return fmt.Errorf("DmsgService->RegistCustomStreamProtocol: protocol %s is already exist", customProtocolID)
	}
	d.customProtocolInfoList[customProtocolID] = &dmsgClientCommon.CustomProtocolInfo{
		StreamProtocol: clientProtocol.NewCustomProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), customProtocolID, d),
		Client:         client,
	}
	client.SetCtx(d.BaseService.GetCtx())
	client.SetService(d)
	return nil
}

func (d *DmsgService) UnregistCustomStreamProtocol(client customProtocol.CustomProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customProtocolInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("DmsgService->UnregistCustomStreamProtocol: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customProtocolInfoList[customProtocolID] = &dmsgClientCommon.CustomProtocolInfo{
		StreamProtocol: clientProtocol.NewCustomProtocol(d.BaseService.GetCtx(), d.BaseService.GetHost(), customProtocolID, d),
		Client:         client,
	}
	return nil
}
