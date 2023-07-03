package client

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	commonKeyUtil "github.com/tinyverse-web3/tvbase/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	clientProtocol "github.com/tinyverse-web3/tvbase/dmsg/client/protocol"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	keyUtil "github.com/tinyverse-web3/tvutil/key"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ProtocolProxy struct {
	readMailboxMsgPrtocol *common.StreamProtocol
	createMailboxProtocol *common.StreamProtocol
	releaseMailboxPrtocol *common.StreamProtocol
	seekMailboxProtocol   *common.PubsubProtocol
	sendMsgPrtocol        *clientProtocol.SendMsgProtocol
}

type DmsgService struct {
	dmsg.DmsgService
	ProtocolProxy
	DestUsers map[string]*common.DestUserInfo
	// client
	SrcUsers               map[string]*common.SrcUserInfo
	CurSrcUserKey          *common.SrcUserKey
	OnReceiveMsg           common.OnReceiveMsg
	customProtocolInfoList map[string]*common.CustomProtocolInfo
}

func CreateService(nodeService tvCommon.NodeService) (*DmsgService, error) {

	d := &DmsgService{}
	err := d.Init(nodeService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DmsgService) Init(nodeService tvCommon.NodeService) error {
	err := d.DmsgService.Init(nodeService)
	if err != nil {
		return err
	}

	// stream protocol
	d.createMailboxProtocol = clientProtocol.NewCreateMailboxProtocol(d.NodeService.GetCtx(), d.NodeService.GetHost(), d)
	d.releaseMailboxPrtocol = clientProtocol.NewReleaseMailboxProtocol(d.NodeService.GetCtx(), d.NodeService.GetHost(), d)
	d.readMailboxMsgPrtocol = clientProtocol.NewReadMailboxMsgProtocol(d.NodeService.GetCtx(), d.NodeService.GetHost(), d)

	// pubsub protocol
	d.seekMailboxProtocol = clientProtocol.NewSeekMailboxProtocol(d.NodeService.GetCtx(), d.NodeService.GetHost(), d, d)
	d.sendMsgPrtocol = clientProtocol.NewSendMsgProtocol(d.NodeService.GetHost(), d, d)

	d.SrcUsers = make(map[string]*common.SrcUserInfo)
	d.DestUsers = make(map[string]*common.DestUserInfo)

	d.customProtocolInfoList = make(map[string]*common.CustomProtocolInfo)
	return nil
}

func (d *DmsgService) setCurSrcUserKey(userID []byte) error {
	//var err error
	//d.CurSrcUserKey, err = d.newSrcUserKey(seed)
	//if err != nil {
	//	return err
	//}
	publicKey, err := keyUtil.ECDSAProtoBufToPublicKey(userID)
	if err != nil {
		dmsgLog.Logger.Errorf("Public key is not ECDSA KEY")
		return err
	}
	d.CurSrcUserKey = &common.SrcUserKey{
		Prikey:    nil,
		Pubkey:    publicKey,
		PrikeyHex: "",
		PubkeyHex: keyUtil.TranslateKeyProtoBufToString(userID),
	}

	return nil
}

/*
func (d *DmsgService) newSrcUserKey(seed string) (*common.SrcUserKey, error) {
	prikey, _, err := keyutil.GenerateEcdsaKey(seed)
	if err != nil {
		return nil, err
	}
	var ret = &common.SrcUserKey{
		Prikey:    prikey,
		Pubkey:    &prikey.PublicKey,
		PrikeyHex: hex.EncodeToString(crypto.FromECDSA(prikey)),
		PubkeyHex: hex.EncodeToString(crypto.FromECDSAPub(&prikey.PublicKey)),
	}
	return ret, nil
}
*/

func (d *DmsgService) initMailbox() error {
	dmsgLog.Logger.Info("initMailbox ...")
	var err error
	userPubkey := d.CurSrcUserKey.PubkeyHex
	pubsub := d.getSrcUserInfo(userPubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("DmsgService->initMailbox: current user pubic key %v is not exist", userPubkey)
		return err
	}

	err = d.seekMailboxProtocol.Request(userPubkey)
	if err != nil {
		dmsgLog.Logger.Errorf(err.Error())
		return err
	}

	select {
	case result := <-pubsub.MailboxCreateSignal:
		if result {
			dmsgLog.Logger.Info("DmsgService->initMailbox: Seek found the mailbox has existed, skip create mailbox")
			return nil
		}
	case <-time.After(time.Second * 3):
		dmsgLog.Logger.Errorf("After 3s, no seek info, will create a new mailbox")
		go d.createMailbox(userPubkey)
		return nil
	case <-d.NodeService.GetCtx().Done():
		dmsgLog.Logger.Errorf("NodeService.GetCtx().Done()")
		return nil
	}

	dmsgLog.Logger.Info("initMailbox done.")
	return nil
}

func (d *DmsgService) createMailbox(userPubkey string) {
	dmsgLog.Logger.Info("createMailbox ...")
	find := false
	pubsub := d.getSrcUserInfo(userPubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("DmsgService->createMailbox: current user pubic key %v is not exist", userPubkey)
		return
	}

	servicePeerList, err := d.NodeService.GetAvailableServicePeerList()
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
		case result := <-pubsub.MailboxCreateSignal:
			if result {
				return
			} else {
				continue
			}
		case <-time.After(time.Second * 3):
			continue
		case <-d.NodeService.GetCtx().Done():
			return
		}
	}
	if !find {
		dmsgLog.Logger.Error("DmsgService->createMailbox: no available mailbox service peers found")
		return
	}
	dmsgLog.Logger.Info("createMailbox done.")
}

func (d *DmsgService) subscribeCurSrcUser() error {
	dmsgLog.Logger.Info("subscribeCurSrcUser ...")
	pubsub := d.getSrcUserInfo(d.CurSrcUserKey.PubkeyHex)
	if pubsub != nil {
		dmsgLog.Logger.Errorf("DmsgService->subscribeCurSrcUser: public key(%s) pubsub already exist", d.CurSrcUserKey.PubkeyHex)
		return fmt.Errorf("DmsgService->subscribeCurSrcUser: public key(%s) pubsub already exist", d.CurSrcUserKey.PubkeyHex)
	}

	err := d.subscribeSrcUser(d.CurSrcUserKey)
	if err != nil {
		dmsgLog.Logger.Error(err)
		return err
	}
	pubsub = d.getSrcUserInfo(d.CurSrcUserKey.PubkeyHex)
	go d.readUserPubsub(pubsub.UserSub)
	return nil
}

func (d *DmsgService) subscribeDestUser(userPubKey string) error {
	dmsgLog.Logger.Info("subscribeDestUser ...")
	var err error
	userTopic, err := d.Pubsub.Join(userPubKey)
	if err != nil {
		dmsgLog.Logger.Error(err)
		return err
	}
	userSub, err := userTopic.Subscribe()
	if err != nil {
		dmsgLog.Logger.Error(err)
		return err
	}
	go d.NodeService.DiscoverRendezvousPeers()

	pubsub := &common.DestUserInfo{}
	pubsub.UserTopic = userTopic
	pubsub.UserSub = userSub
	d.DestUsers[userPubKey] = pubsub

	dmsgLog.Logger.Info("subscribeDestUser done.")
	return nil
}

func (d *DmsgService) unSubscribeDestUser(userPubKey string) error {
	dmsgLog.Logger.Info("unSubscribeDestUser ...")
	userPubsub := d.DestUsers[userPubKey]
	if userPubsub != nil {
		err := userPubsub.UserTopic.Close()
		if err != nil {
			dmsgLog.Logger.Warnln(err)
		}
		userPubsub.UserSub.Cancel()
		delete(d.DestUsers, userPubKey)
	}
	dmsgLog.Logger.Info("unSubscribeDestUser ...")
	return nil
}

func (d *DmsgService) unSubscribeDestUsers() error {
	for userPubKey := range d.DestUsers {
		d.unSubscribeDestUser(userPubKey)
	}
	return nil
}

func (d *DmsgService) subscribeSrcUser(userKey *common.SrcUserKey) error {
	var err error
	userTopic, err := d.Pubsub.Join(userKey.PubkeyHex)
	if err != nil {
		dmsgLog.Logger.Error(err)
		return err
	}

	userSub, err := userTopic.Subscribe()
	if err != nil {
		dmsgLog.Logger.Error(err)
		return err
	}
	go d.NodeService.DiscoverRendezvousPeers()

	userInfo := &common.SrcUserInfo{}
	userInfo.UserTopic = userTopic
	userInfo.UserSub = userSub
	userInfo.MailboxCreateSignal = make(chan bool)

	userInfo.MsgRWMutex = sync.RWMutex{}
	userInfo.UserKey = userKey
	d.SrcUsers[userKey.PubkeyHex] = userInfo
	return nil
}

func (d *DmsgService) unSubscribeSrcUser(userPubKey string) error {
	pubsub := d.SrcUsers[userPubKey]
	if pubsub != nil {
		close(pubsub.MailboxCreateSignal)
		err := pubsub.UserTopic.Close()
		if err != nil {
			dmsgLog.Logger.Warnln(err)
		}
		pubsub.UserSub.Cancel()
		delete(d.SrcUsers, userPubKey)
	}
	return nil
}

func (d *DmsgService) unSubscribeSrcUsers() error {
	for userPubKey := range d.SrcUsers {
		d.unSubscribeSrcUser(userPubKey)
	}
	return nil
}

func (d *DmsgService) getSrcUserInfo(userPubkey string) *common.SrcUserInfo {
	return d.SrcUsers[userPubkey]
}

func (d *DmsgService) getDestUserInfo(userPubkey string) *common.DestUserInfo {
	return d.DestUsers[userPubkey]
}

func (d *DmsgService) readUserPubsub(subscription *pubsub.Subscription) {
	for {
		m, err := subscription.Next(d.NodeService.GetCtx())
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->readUserPubsub: subscription.Next happen err, %v", err)
			return
		}

		if d.NodeService.GetHost().ID() == m.ReceivedFrom {
			continue
		}

		dmsgLog.Logger.Debugf("DmsgService->readUserPubsub: user pubsub msg, topic:%s, receivedFrom:%v", *m.Topic, m.ReceivedFrom)

		protocolID, protocolIDLen, err := d.CheckPubsubData(m.Data)
		if err != nil {
			dmsgLog.Logger.Error(err)
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

func (d *DmsgService) InitUser(userID []byte) error {
	dmsgLog.Logger.Info("InitUser...")
	err := d.setCurSrcUserKey(userID)
	if err != nil {
		dmsgLog.Logger.Error(err)
		return err
	}
	err = d.subscribeCurSrcUser()
	if err != nil {
		dmsgLog.Logger.Error(err)
		return err
	}
	err = d.initMailbox()
	if err != nil {
		dmsgLog.Logger.Error(err)
		return err
	}
	dmsgLog.Logger.Info("InitUser done.")
	return err
}

func (d *DmsgService) IsExistSrcUser(userPubkey string) bool {
	return d.getSrcUserInfo(userPubkey) != nil
}

func (d *DmsgService) IsExistDestUser(userPubkey string) bool {
	return d.getDestUserInfo(userPubkey) != nil
}

func (d *DmsgService) GetCurUserKey() (string, string) {
	return d.CurSrcUserKey.PrikeyHex, d.CurSrcUserKey.PubkeyHex
}

func (d *DmsgService) DeleteCurUserMsg(destPubkey string) error {
	userPubkey := d.CurSrcUserKey.PubkeyHex
	pubsub := d.getSrcUserInfo(userPubkey)
	if pubsub == nil {
		dmsgLog.Logger.Warnf("DmsgService->DeleteCurUserMsg: cannot find src user pubic key %s" + userPubkey)
		return fmt.Errorf("DmsgService->DeleteCurUserMsg: cannot find src user pubic key %s" + userPubkey)
	}
	pubsub.MsgRWMutex.Lock()
	defer pubsub.MsgRWMutex.Unlock()

	prefixStr := d.GetBasicToMsgPrefix(userPubkey, destPubkey)
	var query = query.Query{
		Prefix:   prefixStr,
		KeysOnly: true,
	}
	results, err := d.Datastore.Query(d.NodeService.GetCtx(), query)
	if err != nil {
		return err
	}
	defer results.Close()

	keys := make([]*ds.Key, 0)
	for result := range results.Next() {
		key := ds.NewKey(result.Key)
		keys = append(keys, &key)
	}
	for _, key := range keys {
		d.Datastore.Delete(d.NodeService.GetCtx(), *key)
	}
	return nil
}

func (d *DmsgService) GetUserMsgList(destPubkey string) ([]common.UserMsg, error) {
	userPubkey := d.CurSrcUserKey.PubkeyHex
	var userMsgs []common.UserMsg = make([]common.UserMsg, 0)

	pubkey := userPubkey
	pubsub := d.getSrcUserInfo(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Warnf("DmsgService->GetUserMsgList: cannot find src user public key %s", pubkey)
		return nil, fmt.Errorf("DmsgService->GetUserMsgList: cannot find src user public key %s", pubkey)
	}

	pubsub.MsgRWMutex.RLock()
	defer pubsub.MsgRWMutex.RUnlock()

	prefixStr := d.GetBasicToMsgPrefix(userPubkey, destPubkey)
	var query = query.Query{
		Prefix: prefixStr,
	}
	results, err := d.Datastore.Query(d.NodeService.GetCtx(), query)
	if err != nil {
		return nil, err
	}
	defer results.Close()

	for result := range results.Next() {
		decryptMsgContent, err := commonKeyUtil.DecryptWithPrikey(pubsub.UserKey.Prikey, result.Value)
		if err != nil {
			dmsgLog.Logger.Error(err)
			return nil, err
		}
		fields := strings.Split(result.Key, dmsg.MsgKeyDelimiter)
		if len(fields) < dmsg.MsgFieldsLen {
			dmsgLog.Logger.Errorf("DmsgService->GetUserMsgList: msg key fields len not enough:%v", result.Key)
			return nil, fmt.Errorf("DmsgService->GetUserMsgList: msg key fields len not enough:%v", result.Key)
		}

		timeStamp, err := strconv.ParseInt(fields[dmsg.MsgTimeStampIndex], 10, 64)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->GetUserMsgList: msg timeStamp parse err:%v", err)
			return nil, fmt.Errorf("DmsgService->GetUserMsgList: msg timeStamp parse err:%v", err)
		}
		userMsgs = append(userMsgs, common.UserMsg{
			SrcUserPubkey:  fields[dmsg.MsgSrcUserPubKeyIndex],
			DestUserPubkey: fields[dmsg.MsgDestUserPubKeyIndex],
			Direction:      fields[dmsg.MsgDirectionIndex],
			ID:             fields[dmsg.MsgIDIndex],
			TimeStamp:      timeStamp,
			MsgContent:     string(decryptMsgContent),
		})
		dmsgLog.Logger.Debugf("DmsgService->GetUserMsgList: key:%v, value:%v", result.Key, decryptMsgContent)
	}
	sort.Sort(common.UserMsgByTimeStamp(userMsgs))
	return userMsgs, nil
}

// prepare the message data for send
func (d *DmsgService) PreSendMsg(receiverID string, msgContent []byte) (*pb.SendMsgReq, []byte, error) {
	return d.sendMsgPrtocol.PreRequest(d.CurSrcUserKey.PubkeyHex, receiverID, msgContent)
}

func (d *DmsgService) SendMsg(requestMsg *pb.SendMsgReq, sign []byte) error {
	d.sendMsgPrtocol.Request(requestMsg, sign)
	return nil
}

/*
func (d *DmsgService) SendMsg(destUserPubkeyHex string, msgContent []byte) error {
	var msgData *dmsg.SendMsgData = &dmsg.SendMsgData{
		SrcUserPubkeyHex:  d.CurSrcUserKey.PubkeyHex,
		DestUserPubkeyHex: destUserPubkeyHex,
		SrcUserPrikey:     d.CurSrcUserKey.Prikey,
		SrcUserPubkey:     d.CurSrcUserKey.Pubkey,
		MsgContent:        msgContent,
	}
	err := d.sendMsgPrtocol.Request(msgData)
	if err != nil {
		dmsgLog.Logger.Warnf(err.Error())
		return err
	}

	return nil
}
*/

func (d *DmsgService) SetOnReceiveMsg(onReceiveMsg common.OnReceiveMsg) error {
	d.OnReceiveMsg = onReceiveMsg
	return nil
}

func (d *DmsgService) SubscribeDestUser(pubkey string) error {
	destUserInfo := d.getDestUserInfo(pubkey)
	if destUserInfo != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeDestUser: user public key(%s) pubsub already exist", pubkey)
		return fmt.Errorf("DmsgService->SubscribeDestUser: user public key(%s) pubsub already exist", pubkey)
	}
	err := d.subscribeDestUser(pubkey)
	if err != nil {
		return err
	}
	destUserInfo = d.getDestUserInfo(pubkey)
	go d.readUserPubsub(destUserInfo.UserSub)
	return nil
}

func (d *DmsgService) UnSubscribeDestUser(destPubkey string) error {
	pubsub := d.getDestUserInfo(destPubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("DmsgService->UnSubscribeDestUser: public key(%s) pubsub is not exist", destPubkey)
		return fmt.Errorf("DmsgService->UnSubscribeDestUser: public key(%s) pubsub is not exist", destPubkey)
	}
	err := d.unSubscribeDestUser(destPubkey)
	if err != nil {
		return err
	}
	return nil
}

// StreamProtocolCallback interface
func (d *DmsgService) OnCreateMailboxResponse(protoData protoreflect.ProtoMessage) (interface{}, error) {
	response, ok := protoData.(*pb.CreateMailboxRes)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", protoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxRes", protoData)
	}
	pubsub := d.getSrcUserInfo(response.BasicData.DestPubkey)
	if pubsub == nil {
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
		if pubsub.MailboxPeerId == "" {
			pubsub.MailboxPeerId = response.BasicData.PeerId
		} else if response.BasicData.PeerId != pubsub.MailboxPeerId {
			err = d.releaseMailboxPrtocol.Request(peerId, response.BasicData.DestPubkey)
			if err != nil {
				return nil, err
			}
		}
		pubsub.MailboxCreateSignal <- true
	default: // < 0 service no finish create mailbox
		dmsgLog.Logger.Warnf("DmsgService->OnCreateMailboxResponse: RetCode(%v) fail", response.RetCode)
		pubsub.MailboxCreateSignal <- false
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
	dmsgLog.Logger.Info("DmsgService->OnReadMailboxMsgResponse: new message from mailbox...")
	response, ok := protoData.(*pb.ReadMailboxMsgRes)
	if response == nil || !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: cannot convert %v to *pb.ReadMailboxMsgRes", protoData)
		return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: cannot convert %v to *pb.ReadMailboxMsgRes", protoData)
	}

	pubkey := d.CurSrcUserKey.PubkeyHex
	pubsub := d.getSrcUserInfo(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: cannot find src user pubuc key %s", pubkey)
		return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: cannot find src user pubuc key %s", pubkey)
	}

	pubsub.MsgRWMutex.RLock()
	defer pubsub.MsgRWMutex.RUnlock()

	dmsgLog.Logger.Infof("DmsgService->OnReadMailboxMsgResponse: found (%d) new message...", len(response.MailboxMsgDatas))
	for _, mailboxMsgData := range response.MailboxMsgDatas {

		dmsgLog.Logger.Infof("DmsgService->OnReadMailboxMsgResponse: new message Key = %s", mailboxMsgData.Key)
		msgContent := mailboxMsgData.MsgContent
		//err := d.Datastore.Put(d.NodeService.GetCtx(), ds.NewKey(mailboxMsgData.Key), msgContent)

		fields := strings.Split(mailboxMsgData.Key, dmsg.MsgKeyDelimiter)
		if len(fields) < dmsg.MsgFieldsLen {
			dmsgLog.Logger.Errorf("DmsgService->GetUserMsgList: msg key fields len not enough:%v", mailboxMsgData.Key)
			return nil, fmt.Errorf("DmsgService->GetUserMsgList: msg key fields len not enough:%v", mailboxMsgData.Key)
		}

		timeStamp, err := strconv.ParseInt(fields[dmsg.MsgTimeStampIndex], 10, 64)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->GetUserMsgList: msg timeStamp parse err:%v", err)
			return nil, fmt.Errorf("DmsgService->GetUserMsgList: msg timeStamp parse err:%v", err)
		}

		//dmsgLog.Logger.Debugf("DmsgService->GetUserMsgList: key:%v, value:%v", mailboxMsgData.Key, msgContent)

		destPubkey := fields[dmsg.MsgSrcUserPubKeyIndex]
		srcPubkey := fields[dmsg.MsgDestUserPubKeyIndex]
		msgID := fields[dmsg.MsgIDIndex]

		dmsgLog.Logger.Info("DmsgService->OnReadMailboxMsgResponse: From = ", srcPubkey)
		dmsgLog.Logger.Info("DmsgService->OnReadMailboxMsgResponse: To = ", destPubkey)
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
	pubsub := d.getSrcUserInfo(pubkey)
	if pubsub == nil {
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
	if pubsub.MailboxPeerId == "" {
		pubsub.MailboxPeerId = response.BasicData.PeerId
		pubsub.MailboxCreateSignal <- true
		return nil, nil
	} else if response.PeerId != pubsub.MailboxPeerId {
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

func (d *DmsgService) OnSendMsgResquest(protoMsg protoreflect.ProtoMessage, protoData []byte) (interface{}, error) {
	request, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnSendMsgResquest: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return nil, fmt.Errorf("DmsgService->OnSendMsgResquest: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}
	pubkey := request.BasicData.DestPubkey
	pubsub := d.getSrcUserInfo(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("DmsgService->OnSendMsgResquest: cannot find src user public key %v", pubkey)
		return nil, fmt.Errorf("DmsgService->OnSendMsgResquest: cannot find src user public key %v", pubkey)
	}

	//err := d.SaveUserMsg(protoMsg, dmsg.MsgDirection.From)
	//sendMsgReq, ok := protoMsg.(*pb.SendMsgReq)
	srcPubkey := request.SrcPubkey
	destPubkey := request.BasicData.DestPubkey
	d.OnReceiveMsg(
		srcPubkey,
		destPubkey,
		request.MsgContent,
		request.BasicData.Timestamp,
		request.BasicData.Id,
		dmsg.MsgDirection.From)
	/*
		if err != nil {
			return nil, err
		}
	*/
	return nil, nil
}

// ClientService interface
func (d *DmsgService) PublishProtocol(protocolID pb.ProtocolID, userPubkey string,
	protocolData []byte, pubsubSource common.PubsubSourceType) error {
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
	case common.PubsubSource.DestUser:
		userPubsub := d.getDestUserInfo(userPubkey)
		if userPubsub == nil {
			dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: cannot find dest user pubsub key %s", userPubkey)
			return fmt.Errorf("DmsgService->PublishProtocol: cannot find dest user pubsub key %s", userPubkey)
		}
		err := userPubsub.UserTopic.Publish(d.NodeService.GetCtx(), pubsubData)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: publish protocol error: %v", err)
			return fmt.Errorf("DmsgService->PublishProtocol: publish protocol error: %v", err)
		}
	case common.PubsubSource.SrcUser:
		userPubsub := d.getSrcUserInfo(userPubkey)
		if userPubsub == nil {
			dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: cannot find src user pubsub key %s", userPubkey)
			return fmt.Errorf("DmsgService->PublishProtocol: cannot find src user pubsub key %s", userPubkey)
		}
		err := userPubsub.UserTopic.Publish(d.NodeService.GetCtx(), pubsubData)
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

func (d *DmsgService) SaveUserMsg(protoMsg protoreflect.ProtoMessage, msgDirection string) error {
	sendMsgReq, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->SaveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return fmt.Errorf("DmsgService->SaveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}

	key := ""
	switch msgDirection {
	case dmsg.MsgDirection.To:
		key = d.GetFullToMsgPrefix(sendMsgReq)
	case dmsg.MsgDirection.From:
		key = d.GetFullFromMsgPrefix(sendMsgReq)
	default:
		return errors.New("DmsgService->SaveUserMsg: invalid msgDirection")
	}

	pubkey := d.CurSrcUserKey.PubkeyHex
	pubsub := d.getSrcUserInfo(pubkey)
	if pubsub == nil {
		dmsgLog.Logger.Errorf("DmsgService->SaveUserMsg: annot find src user public key %v", pubkey)
		return fmt.Errorf("DmsgService->SaveUserMsg: annot find src user public key %v", pubkey)
	}

	pubsub.MsgRWMutex.RLock()
	defer pubsub.MsgRWMutex.RUnlock()
	err := d.Datastore.Put(d.NodeService.GetCtx(), ds.NewKey(key), sendMsgReq.MsgContent)
	if err != nil {
		return err
	}

	decryptMsgContent, err := commonKeyUtil.DecryptWithPrikey(pubsub.UserKey.Prikey, sendMsgReq.MsgContent)
	if err != nil {
		return err
	}
	srcPubkey := sendMsgReq.BasicData.DestPubkey
	destPubkey := sendMsgReq.SrcPubkey
	if msgDirection == dmsg.MsgDirection.To {
		srcPubkey = sendMsgReq.SrcPubkey
		destPubkey = sendMsgReq.BasicData.DestPubkey
	}
	if d.OnReceiveMsg != nil {
		d.OnReceiveMsg(
			srcPubkey,
			destPubkey,
			decryptMsgContent,
			sendMsgReq.BasicData.Timestamp,
			sendMsgReq.BasicData.Id,
			msgDirection)
	} else {
		dmsgLog.Logger.Debugf("DmsgService->SaveUserMsg: %v send msg to %v, msgContent:%v", srcPubkey, destPubkey, decryptMsgContent)
	}
	return err
}

// cmd protocol
func (d *DmsgService) RequestCustomProtocol(customProtocolId string, content []byte) error {
	find := false
	protocolInfo := d.customProtocolInfoList[customProtocolId]
	if protocolInfo == nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomProtocol: protocol %s is not exist", customProtocolId)
		return fmt.Errorf("DmsgService->RequestCustomProtocol: protocol %s is not exist", customProtocolId)
	}

	servicePeerList, err := d.NodeService.GetAvailableServicePeerList()
	if err != nil {
		dmsgLog.Logger.Error(err)
	}

	for _, peerInfo := range servicePeerList {
		err := protocolInfo.StreamProtocol.RequestCustomProtocol(peerInfo, d.CurSrcUserKey.PubkeyHex, customProtocolId, content)
		if err != nil {
			dmsgLog.Logger.Errorf("DmsgService->RequestCustomProtocol: err: %v, servicePeerInfo: %v, user public key: %s, content: %v",
				err, peerInfo, d.CurSrcUserKey.PubkeyHex, content)
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
	d.customProtocolInfoList[customProtocolID] = &common.CustomProtocolInfo{
		StreamProtocol: clientProtocol.NewCustomProtocol(d.NodeService.GetCtx(), d.NodeService.GetHost(), customProtocolID, d),
		Client:         client,
	}
	client.SetCtx(d.NodeService.GetCtx())
	client.SetService(d)
	return nil
}

func (d *DmsgService) UnregistCustomStreamProtocol(client customProtocol.CustomProtocolClient) error {
	customProtocolID := client.GetProtocolID()
	if d.customProtocolInfoList[customProtocolID] == nil {
		dmsgLog.Logger.Warnf("DmsgService->UnregistCustomStreamProtocol: protocol %s is not exist", customProtocolID)
		return nil
	}
	d.customProtocolInfoList[customProtocolID] = &common.CustomProtocolInfo{
		StreamProtocol: clientProtocol.NewCustomProtocol(d.NodeService.GetCtx(), d.NodeService.GetHost(), customProtocolID, d),
		Client:         client,
	}
	return nil
}
