package client

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/dmsg"
	dmsgClientCommon "github.com/tinyverse-web3/tvbase/dmsg/client/common"
	clientProtocol "github.com/tinyverse-web3/tvbase/dmsg/client/protocol"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
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
	user         *dmsgUser.LightDmsgUser
	onReceiveMsg dmsgClientCommon.OnReceiveMsg

	destUserList                 map[string]*dmsgUser.LightDestUser
	pubChannelList               map[string]*dmsgUser.PubChannel
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

	d.destUserList = make(map[string]*dmsgUser.LightDestUser)
	d.pubChannelList = make(map[string]*dmsgUser.PubChannel)

	d.customStreamProtocolInfoList = make(map[string]*dmsgClientCommon.CustomStreamProtocolInfo)
	d.customPubsubProtocolInfoList = make(map[string]*dmsgClientCommon.CustomPubsubProtocolInfo)
	return nil
}

func (d *DmsgService) getDestUserInfo(userPubkey string) *dmsgUser.LightDestUser {
	return d.destUserList[userPubkey]
}

// for sdk
func (d *DmsgService) Start() error {

	return nil
}

func (d *DmsgService) Stop() error {
	d.UnSubscribeSrcUser()
	d.UnSubscribeDestUsers()
	return nil
}

func (d *DmsgService) InitUser(userPubkeyData []byte, getSigCallback dmsgKey.GetSigCallback, done chan any) error {
	dmsgLog.Logger.Debug("DmsgService->InitUser begin")
	userPubkey := keyUtil.TranslateKeyProtoBufToString(userPubkeyData)
	err := d.SubscribeSrcUser(userPubkey, getSigCallback)
	if err != nil {
		return err
	}

	initMailbox := func() {
		dmsgLog.Logger.Debug("DmsgService->InitUser->initMailbox begin")
		if d == nil {
			dmsgLog.Logger.Errorf("DmsgService->InitUser: DmsgService is nil")
			done <- fmt.Errorf("DmsgService is nil")
			return
		}
		if d.user == nil {
			dmsgLog.Logger.Errorf("DmsgService->InitUser: SrcUserInfo is nil")
			done <- fmt.Errorf("SrcUserInfo is nil")
			return
		}
		_, seekMailboxDoneChan, err := d.seekMailboxProtocol.Request(d.user.Key.PubkeyHex, d.user.Key.PubkeyHex)
		if err != nil {
			done <- err
			return
		}
		select {
		case seekMailboxResponseProtoData := <-seekMailboxDoneChan:
			dmsgLog.Logger.Debugf("DmsgService->InitUser: seekMailboxProtoData: %+v", seekMailboxResponseProtoData)
			response, ok := seekMailboxResponseProtoData.(*pb.SeekMailboxRes)
			if !ok || response == nil {
				dmsgLog.Logger.Errorf("DmsgService->InitUser: seekMailboxProtoData is not SeekMailboxRes")
				// skip seek when seek mailbox quest fail (server err), create a new mailbox
			}
			if response.RetCode.Code < 0 {
				dmsgLog.Logger.Errorf("DmsgService->InitUser: seekMailboxProtoData fail")
				// skip seek when seek mailbox quest fail, create a new mailbox
			} else {
				dmsgLog.Logger.Debugf("DmsgService->InitUser: seekMailboxProtoData success")
				done <- nil

				go d.releaseUnusedMailbox(response.BasicData.PeerID, userPubkey)
				return
			}
		case <-time.After(3 * time.Second):
			dmsgLog.Logger.Debugf("DmsgService->InitUser: time.After 3s, create new mailbox")
			// begin create new mailbox

			hostId := d.BaseService.GetHost().ID().String()
			servicePeerList, err := d.BaseService.GetAvailableServicePeerList(hostId)
			if err != nil {
				dmsgLog.Logger.Errorf("DmsgService->InitUser: getAvailableServicePeerList error: %v", err)
				done <- err
				return
			}

			for _, servicePeerID := range servicePeerList {
				dmsgLog.Logger.Debugf("DmsgService->InitUser: servicePeerID: %v", servicePeerID)
				_, createMailboxDoneChan, err := d.createMailboxProtocol.Request(servicePeerID, userPubkey)
				if err != nil {
					dmsgLog.Logger.Errorf("DmsgService->InitUser: createMailboxProtocol.Request error: %v", err)
					continue
				}

				select {
				case createMailboxResponseProtoData := <-createMailboxDoneChan:
					dmsgLog.Logger.Debugf("DmsgService->InitUser: createMailboxResponseProtoData: %+v", createMailboxResponseProtoData)
					response, ok := createMailboxResponseProtoData.(*pb.CreateMailboxRes)
					if !ok || response == nil {
						dmsgLog.Logger.Errorf("DmsgService->InitUser: createMailboxDoneChan is not CreateMailboxRes")
						continue
					}

					switch response.RetCode.Code {
					case 0, 1:
						dmsgLog.Logger.Debugf("DmsgService->InitUser: createMailboxProtocol success")
						done <- nil
						return
					default:
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

			dmsgLog.Logger.Error("DmsgService->InitUser: no available service peers")
			done <- fmt.Errorf("DmsgService->InitUser: no available service peers")
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

func (d *DmsgService) IsExistDestUser(userPubkey string) bool {
	return d.getDestUserInfo(userPubkey) != nil
}

func (d *DmsgService) GetUserPubkeyHex() (string, error) {
	if d.user == nil {
		return "", fmt.Errorf("MailboxService->GetUserPubkeyHex: user is nil")
	}
	return d.user.Key.PubkeyHex, nil
}

func (d *DmsgService) SubscribeSrcUser(pubkey string, getSig dmsgKey.GetSigCallback) error {
	dmsgLog.Logger.Debugf("DmsgService->SubscribeSrcUser begin\npubkey: %s", pubkey)
	if d.user != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: user isn't nil")
		return fmt.Errorf("DmsgService->SubscribeSrcUser: user isn't nil")
	}

	if d.IsExistDestUser(pubkey) {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: pubkey is already exist in destUserList")
		return fmt.Errorf("DmsgService->SubscribeSrcUser: pubkey is already exist in destUserList")
	}

	user, err := dmsgUser.NewUser(d.BaseService.GetCtx(), d.Pubsub, pubkey, getSig)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeSrcUser: NewUser error: %v", err)
		return err
	}

	d.user = &dmsgUser.LightDmsgUser{
		User:          *user,
		ServicePeerID: "",
	}

	// go d.BaseService.DiscoverRendezvousPeers()
	go d.HandleProtocolWithPubsub(&d.user.User)

	dmsgLog.Logger.Debugf("DmsgService->SubscribeSrcUser end")
	return nil
}

func (d *DmsgService) UnSubscribeSrcUser() error {
	dmsgLog.Logger.Debugf("DmsgService->UnSubscribeSrcUser begin")
	if d.user == nil {
		dmsgLog.Logger.Errorf("DmsgService->UnSubscribeSrcUser: userPubkey is not exist in destUserInfoList")
		return fmt.Errorf("DmsgService->UnSubscribeSrcUser: userPubkey is not exist in destUserInfoList")
	}
	dmsgLog.Logger.Debugf("DmsgService->UnSubscribeSrcUser:\nsrcUserInfo: %+v", d.user)

	d.user.Close()
	d.user = nil
	dmsgLog.Logger.Debugf("DmsgService->UnSubscribeSrcUser end")
	return nil
}

// dest user
func (d *DmsgService) SubscribeDestUser(pubkey string) error {
	dmsgLog.Logger.Debug("DmsgService->subscribeDestUser begin\npubkey: %s", pubkey)
	if d.IsExistDestUser(pubkey) {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeDestUser: pubkey is already exist in destUserList")
		return fmt.Errorf("DmsgService->SubscribeDestUser: pubkey is already exist in destUserList")
	}
	if d.user != nil && d.user.Key.PubkeyHex == pubkey {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeDestUser: pubkey equal to user pubkey")
		return fmt.Errorf("DmsgService->SubscribeDestUser: pubkey equal to user pubkey")
	}

	user, err := dmsgUser.NewUser(d.BaseService.GetCtx(), d.Pubsub, pubkey, nil)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeDestUser: NewUser error: %v", err)
		return err
	}

	destUser := &dmsgUser.LightDestUser{
		User: *user,
	}
	d.destUserList[pubkey] = destUser

	// go d.BaseService.DiscoverRendezvousPeers()
	go d.HandleProtocolWithPubsub(&destUser.User)

	dmsgLog.Logger.Debug("DmsgService->subscribeDestUser end")
	return nil
}

func (d *DmsgService) UnSubscribeDestUser(pubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->UnSubscribeDestUser begin\npubkey: %s", pubkey)

	user := d.getDestUserInfo(pubkey)
	if user == nil {
		dmsgLog.Logger.Errorf("DmsgService->UnSubscribeDestUser: pubkey is not exist in destUserList")
		return fmt.Errorf("DmsgService->UnSubscribeDestUser: pubkey is not exist in destUserList")
	}
	user.Close()
	delete(d.destUserList, pubkey)

	dmsgLog.Logger.Debug("DmsgService->unSubscribeDestUser end")
	return nil
}

func (d *DmsgService) UnSubscribeDestUsers() error {
	for userPubKey := range d.destUserList {
		d.UnSubscribeDestUser(userPubKey)
	}
	return nil
}

// pub channel
func (d *DmsgService) getPubChannelInfo(userPubkey string) *dmsgUser.PubChannel {
	return d.pubChannelList[userPubkey]
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

	user, err := dmsgUser.NewUser(d.BaseService.GetCtx(), d.Pubsub, pubkey, nil)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SubscribeDestUser: NewUser error: %v", err)
		return err
	}

	pubChannel := &dmsgUser.PubChannel{
		User:                 *user,
		LastRequestTimestamp: time.Now().Unix(),
	}
	d.pubChannelList[pubkey] = pubChannel

	err = d.createPubChannelService(pubChannel.Key.PubkeyHex)
	if err != nil {
		user.Close()
		delete(d.pubChannelList, pubkey)
		return err
	}

	// go d.BaseService.DiscoverRendezvousPeers()
	go d.HandleProtocolWithPubsub(&pubChannel.User)

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
	delete(d.pubChannelList, userPubKey)

	dmsgLog.Logger.Debug("DmsgService->UnsubscribePubChannel end")
	return nil
}

func (d *DmsgService) createPubChannelService(pubkey string) error {
	dmsgLog.Logger.Debugf("DmsgService->createPubChannelService begin:\npublic channel key: %s", pubkey)
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
		_, createPubChannelDoneChan, err := d.createPubChannelProtocol.Request(servicePeerID, srcPubkey, pubkey)
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
	if d.user == nil {
		dmsgLog.Logger.Errorf("DmsgService->GetUserSig: user is nil")
		return nil, fmt.Errorf("DmsgService->GetUserSig: user is nil")
	}
	return d.user.GetSig(protoData)
}

func (d *DmsgService) SendMsg(destPubkey string, content []byte) (*pb.SendMsgReq, error) {
	dmsgLog.Logger.Debugf("DmsgService->SendMsg begin:\ndestPubkey: %v", destPubkey)
	sigPubkey := d.user.Key.PubkeyHex
	protoData, _, err := d.sendMsgPubPrtocol.Request(
		sigPubkey,
		destPubkey,
		content,
	)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->SendMsg: %v", err)
		return nil, err
	}
	sendMsgReq, ok := protoData.(*pb.SendMsgReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->SendMsg: protoData is not SendMsgReq")
		return nil, fmt.Errorf("DmsgService->SendMsg: protoData is not SendMsgReq")
	}
	dmsgLog.Logger.Debugf("DmsgService->SendMsg end")
	return sendMsgReq, nil
}

func (d *DmsgService) SetOnReceiveMsg(onReceiveMsg dmsgClientCommon.OnReceiveMsg) {
	d.onReceiveMsg = onReceiveMsg
}

func (d *DmsgService) RequestReadMailbox(timeout time.Duration) ([]dmsg.Msg, error) {
	var msgList []dmsg.Msg
	peerID, err := peer.Decode(d.user.ServicePeerID)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestReadMailbox: peer.Decode error: %v", err)
		return msgList, err
	}
	_, readMailboxDoneChan, err := d.readMailboxMsgPrtocol.Request(peerID, d.user.Key.PubkeyHex)
	if err != nil {
		return msgList, err
	}

	select {
	case responseProtoData := <-readMailboxDoneChan:
		dmsgLog.Logger.Debugf("DmsgService->RequestReadMailbox: responseProtoData: %+v", responseProtoData)
		response, ok := responseProtoData.(*pb.ReadMailboxRes)
		if !ok || response == nil {
			dmsgLog.Logger.Errorf("DmsgService->RequestReadMailbox: readMailboxDoneChan is not ReadMailboxRes")
			return msgList, fmt.Errorf("DmsgService->RequestReadMailbox: readMailboxDoneChan is not ReadMailboxRes")
		}
		dmsgLog.Logger.Debugf("DmsgService->RequestReadMailbox: readMailboxChanDoneChan success")
		msgList, err = d.parseReadMailboxResponse(response, dmsg.MsgDirection.From)
		if err != nil {
			return msgList, err
		}

		return msgList, nil
	case <-time.After(timeout):
		dmsgLog.Logger.Debugf("DmsgService->RequestReadMailbox: timeout")
		return msgList, fmt.Errorf("DmsgService->RequestReadMailbox: timeout")
	case <-d.BaseService.GetCtx().Done():
		dmsgLog.Logger.Debugf("DmsgService->RequestReadMailbox: BaseService.GetCtx().Done()")
		return msgList, fmt.Errorf("DmsgService->RequestReadMailbox: BaseService.GetCtx().Done()")
	}
}

func (d *DmsgService) releaseUnusedMailbox(peerIdHex string, pubkey string) error {
	dmsgLog.Logger.Debug("DmsgService->releaseUnusedMailbox begin")

	peerID, err := peer.Decode(peerIdHex)
	if err != nil {
		dmsgLog.Logger.Warnf("DmsgService->releaseUnusedMailbox: fail to decode peer id: %v", err)
		return err
	}
	_, readMailboxDoneChan, err := d.readMailboxMsgPrtocol.Request(peerID, pubkey)
	if err != nil {
		return err
	}

	select {
	case <-readMailboxDoneChan:
		if d.user.ServicePeerID == "" {
			d.user.ServicePeerID = peerIdHex
		} else if peerIdHex != d.user.ServicePeerID {
			_, releaseMailboxDoneChan, err := d.releaseMailboxPrtocol.Request(peerID, pubkey)
			if err != nil {
				return err
			}
			select {
			case <-releaseMailboxDoneChan:
				dmsgLog.Logger.Debugf("DmsgService->releaseUnusedMailbox: releaseMailboxDoneChan success")
			case <-time.After(time.Second * 3):
				return fmt.Errorf("DmsgService->releaseUnusedMailbox: releaseMailboxDoneChan time out")
			case <-d.BaseService.GetCtx().Done():
				return fmt.Errorf("DmsgService->releaseUnusedMailbox: BaseService.GetCtx().Done()")
			}
		}
	case <-time.After(time.Second * 10):
		return fmt.Errorf("DmsgService->releaseUnusedMailbox: readMailboxDoneChan time out")
	case <-d.BaseService.GetCtx().Done():
		return fmt.Errorf("DmsgService->releaseUnusedMailbox: BaseService.GetCtx().Done()")
	}
	dmsgLog.Logger.Debugf("DmsgService->releaseUnusedMailbox end")
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
	dmsgLog.Logger.Debug("DmsgService->OnCreateMailboxRequest begin\nrequestProtoData: %+v", requestProtoData)
	_, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		dmsgLog.Logger.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxReq", requestProtoData)
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: cannot convert %v to *pb.CreateMailboxReq", requestProtoData)
	}

	dmsgLog.Logger.Debug("DmsgService->OnCreateMailboxRequest end")
	return nil, nil
}

func (d *DmsgService) OnCreateMailboxResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnCreateMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
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

	switch response.RetCode.Code {
	case 0: // new
		fallthrough
	case 1: // exist mailbox
		dmsgLog.Logger.Debug("DmsgService->OnCreateMailboxResponse: mailbox has created, read message from mailbox...")
		err := d.releaseUnusedMailbox(response.BasicData.PeerID, request.BasicData.Pubkey)
		if err != nil {
			return nil, err
		}

	default: // < 0 service no finish create mailbox
		dmsgLog.Logger.Warnf("DmsgService->OnCreateMailboxResponse: RetCode(%v) fail", response.RetCode)
	}

	dmsgLog.Logger.Debug("DmsgService->OnCreateMailboxResponse end")
	return nil, nil
}

func (d *DmsgService) OnCreatePubChannelRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debugf("DmsgService->OnCreatePubChannelRequest: begin\nrequestProtoData: %+v", requestProtoData)

	dmsgLog.Logger.Debugf("DmsgService->OnCreatePubChannelRequest end")
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
	dmsgLog.Logger.Debugf("DmsgService->OnReleaseMailboxRequest: begin\nrequestProtoData: %+v", requestProtoData)

	dmsgLog.Logger.Debugf("DmsgService->OnReleaseMailboxRequest end")
	return nil, nil
}

func (d *DmsgService) OnReleaseMailboxResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnReleaseMailboxResponse begin")
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

	dmsgLog.Logger.Debug("DmsgService->OnReleaseMailboxResponse end")
	return nil, nil
}

func (d *DmsgService) OnReadMailboxMsgRequest(requestProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnReadMailboxMsgRequest: begin\nrequestProtoData: %+v", requestProtoData)

	dmsgLog.Logger.Debugf("DmsgService->OnReadMailboxMsgRequest: end")
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
	dmsgLog.Logger.Debug("DmsgService->OnSeekMailboxRequest begin")

	dmsgLog.Logger.Debug("DmsgService->OnSeekMailboxRequest end")
	return nil, nil
}

func (d *DmsgService) OnSeekMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	dmsgLog.Logger.Debug("DmsgService->OnSeekMailboxResponse begin")

	dmsgLog.Logger.Debugf("DmsgService->OnSeekMailboxResponse end")
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

	if d.onReceiveMsg != nil {
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
func (d *DmsgService) PublishProtocol(ctx context.Context, pubkey string, pid pb.PID, protoData []byte) error {
	var user *dmsgUser.User = nil
	destUser := d.getDestUserInfo(pubkey)
	if destUser == nil {
		if d.user.Key.PubkeyHex != pubkey {
			pubChannel := d.pubChannelList[pubkey]
			if pubChannel == nil {
				dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: cannot find src/dest/pubchannel user Info for key %s", pubkey)
				return fmt.Errorf("DmsgService->PublishProtocol: cannot find src/dest/pubchannel user Info for key %s", pubkey)
			}
			user = &pubChannel.User
		} else {
			user = &d.user.User
		}
	} else {
		user = &destUser.User
	}

	buf, err := dmsgProtocol.GenProtoData(pid, protoData)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: GenProtoData error: %v", err)
		return err
	}

	err = user.Topic.Publish(ctx, buf)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->PublishProtocol: Publish error: %v", err)
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
	_, _, err = protocolInfo.Protocol.Request(
		peerId,
		d.user.Key.PubkeyHex,
		pid,
		content)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->RequestCustomStreamProtocol: err: %v, servicePeerInfo: %v, user public key: %s, content: %v",
			err, peerId, d.user.Key.PubkeyHex, content)
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
