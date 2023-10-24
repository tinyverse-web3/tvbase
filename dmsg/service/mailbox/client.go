package mailbox

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MailboxClient struct {
	MailboxBase
	createMailboxProtocol *dmsgProtocol.MailboxSProtocol
	releaseMailboxPrtocol *dmsgProtocol.MailboxSProtocol
	readMailboxMsgPrtocol *dmsgProtocol.MailboxSProtocol
	seekMailboxProtocol   *dmsgProtocol.MailboxPProtocol
	pubsubMsgProtocol     *dmsgProtocol.PubsubMsgProtocol
	lightMailboxUser      *dmsgUser.LightMailboxUser
	proxyUser             *dmsgUser.LightMailboxUser
	onReceiveMsg          msg.OnReceiveMsg
	stopReadMailbox       chan bool
}

func CreateClient(tvbaseService define.TvBaseService) (*MailboxClient, error) {
	d := &MailboxClient{}
	err := d.Init(tvbaseService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *MailboxClient) Init(tvbaseService define.TvBaseService) error {
	err := d.BaseService.Init(tvbaseService)
	if err != nil {
		return err
	}
	return nil
}

// sdk-common

func (d *MailboxClient) Start(pubkey string, getSig dmsgKey.GetSigCallback) error {
	err := d.SubscribeUser(pubkey, getSig)
	if err != nil {
		return err
	}

	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()
	// stream protocol
	d.createMailboxProtocol = adapter.NewCreateMailboxProtocol(ctx, host, d, d, false, pubkey)
	d.releaseMailboxPrtocol = adapter.NewReleaseMailboxProtocol(ctx, host, d, d, false, pubkey)
	d.readMailboxMsgPrtocol = adapter.NewReadMailboxMsgProtocol(ctx, host, d, d, false, pubkey)

	// pubsub protocol
	d.seekMailboxProtocol = adapter.NewSeekMailboxProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(d.seekMailboxProtocol.Adapter.GetResponsePID(), d.seekMailboxProtocol)
	return nil
}

func (d *MailboxClient) Stop() error {
	log.Debug("MailboxService->Stop begin")
	d.UnregistPubsubProtocol(d.seekMailboxProtocol.Adapter.GetRequestPID())
	d.UnSubscribeUser()

	close(d.stopReadMailbox)
	log.Debug("MailboxService->Stop end")
	return nil
}

func (d *MailboxClient) SetOnReceiveMsg(cb msg.OnReceiveMsg) {
	d.onReceiveMsg = cb
}

// sdk-msg
func (d *MailboxClient) ReadMailbox(timeout time.Duration) ([]msg.ReceiveMsg, error) {
	if d.lightMailboxUser == nil {
		log.Errorf("MailboxClient->ReadMailbox: user is nil")
		return nil, fmt.Errorf("MailboxClient->ReadMailbox: user is nil")
	}
	if d.lightMailboxUser.ServicePeerID == "" {
		log.Errorf("MailboxClient->ReadMailbox: servicePeerID is empty")
		return nil, fmt.Errorf("MailboxClient->ReadMailbox: servicePeerID is empty")
	}
	return d.readMailbox(
		d.lightMailboxUser.ServicePeerID,
		d.lightMailboxUser.Key.PubkeyHex,
		timeout,
		false,
	)
}

// DmsgService

func (d *MailboxClient) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	var target *dmsgUser.Target
	if d.proxyUser != nil && d.proxyUser.Key.PubkeyHex == pubkey {
		target = &d.proxyUser.Target
	}

	if target != nil {
		return target, nil
	}

	var err error
	target, err = d.MailboxBase.GetPublishTarget(pubkey)
	if err != nil {
		return nil, err
	}
	return target, nil
}

// MailboxSpCallback
func (d *MailboxClient) OnCreateMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf("MailboxClient->OnCreateMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	request, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		log.Debugf("MailboxClient->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.CreateMailboxReq")
		// return nil, fmt.Errorf("MailboxClient->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.CreateMailboxReq")
	}
	response, ok := responseProtoData.(*pb.CreateMailboxRes)
	if !ok {
		log.Errorf("MailboxClient->OnCreateMailboxResponse: fail to convert responseProtoData to *pb.CreateMailboxRes")
		return nil, fmt.Errorf("MailboxClient->OnCreateMailboxResponse: fail to convert responseProtoData to *pb.CreateMailboxRes")
	}

	switch response.RetCode.Code {
	case 0: // new
		fallthrough
	case 1: // exist mailbox
		log.Debug("MailboxClient->OnCreateMailboxResponse: mailbox has created, read message from mailbox...")
		err := d.releaseUnusedMailbox(response.BasicData.PeerID, request.BasicData.Pubkey, 30*time.Second)
		if err != nil {
			return nil, err
		}
	case -1:
		log.Warnf("MailboxClient->OnCreateMailboxResponse: fail RetCode: %+v ", response.RetCode)
	default:
		log.Warnf("MailboxClient->OnCreateMailboxResponse: other case RetCode: %+v", response.RetCode)
	}

	log.Debug("MailboxClient->OnCreateMailboxResponse end")
	return nil, nil
}

func (d *MailboxClient) OnReleaseMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debug(
		"MailboxClient->OnReleaseMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	response, ok := responseProtoData.(*pb.ReleaseMailboxRes)
	if !ok {
		log.Errorf("MailboxClient->OnReleaseMailboxResponse: fail to convert responseProtoData to *pb.ReleaseMailboxRes")
		return nil, fmt.Errorf("MailboxClient->OnReleaseMailboxResponse: fail to convert responseProtoData to *pb.ReleaseMailboxRes")
	}
	if response.RetCode.Code < 0 {
		log.Warnf("MailboxClient->OnReleaseMailboxResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("MailboxClient->OnReleaseMailboxResponse: fail RetCode: %+v", response.RetCode)
	}

	log.Debug("MailboxClient->OnReleaseMailboxResponse end")
	return nil, nil
}

func (d *MailboxClient) OnReadMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf("MailboxClient->OnReadMailboxResponse: begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		log.Errorf("MailboxClient->OnReadMailboxResponse: fail to convert responseProtoData to *pb.ReadMailboxMsgRes")
		return nil, fmt.Errorf("MailboxClient->OnReadMailboxResponse: fail to convert responseProtoData to *pb.ReadMailboxMsgRes")
	}

	log.Debugf("MailboxClient->OnReadMailboxResponse: found (%d) new message", len(response.ContentList))

	msgList, err := d.parseReadMailboxResponse(responseProtoData, msg.MsgDirection.From)
	if err != nil {
		return nil, err
	}
	for _, msg := range msgList {
		log.Debugf("MailboxClient->OnReadMailboxResponse: From = %s, To = %s", msg.ReqPubkey, msg.DestPubkey)
		if d.onReceiveMsg != nil {
			d.onReceiveMsg(&msg)
		} else {
			log.Warnf("MailboxClient->OnReadMailboxResponse: callback func onReadMailmsg is nil")
		}
	}

	log.Debug("MailboxClient->OnReadMailboxResponse end")
	return nil, nil
}

// MailboxPpCallback

func (d *MailboxClient) OnSeekMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf(
		"MailboxClient->OnSeekMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	request, ok := requestProtoData.(*pb.SeekMailboxReq)
	if request == nil || !ok {
		log.Errorf("MailboxClient->OnSeekMailboxResponse: fail to convert requestProtoData to *pb.SeekMailboxReq")
		return nil, fmt.Errorf("MailboxClient->OnSeekMailboxResponse: fail to convert requestProtoData to *pb.SeekMailboxReq")
	}
	response, ok := responseProtoData.(*pb.SeekMailboxRes)
	if response == nil || !ok {
		log.Errorf("MailboxClient->OnSeekMailboxResponse: fail to convert responseProtoData to *pb.SeekMailboxRes")
		return nil, fmt.Errorf("MailboxClient->OnSeekMailboxResponse: fail to convert responseProtoData to *pb.SeekMailboxRes")
	}

	if request.BasicData.Pubkey != d.lightMailboxUser.Key.PubkeyHex {
		log.Errorf("MailboxClient->OnSeekMailboxResponse: fail request.BasicData.Pubkey != d.lightMailboxUser.Key.PubkeyHex")
	}

	log.Debug("MailboxClient->OnSeekMailboxResponse end")
	return nil, nil
}

func (d *MailboxClient) OnPubsubMsgResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

// user

func (d *MailboxClient) SubscribeUser(pubkey string, getSig dmsgKey.GetSigCallback) error {
	log.Debugf("MailboxClient->SubscribeUser begin\npubkey: %s", pubkey)
	if d.lightMailboxUser != nil {
		log.Errorf("MailboxClient->SubscribeUser: user isn't nil")
		return fmt.Errorf("MailboxClient->SubscribeUser: user isn't nil")
	}

	target, err := dmsgUser.NewTarget(pubkey, getSig)
	if err != nil {
		log.Errorf("MailboxClient->SubscribeUser: NewUser error: %v", err)
		return err
	}

	err = target.InitPubsub(pubkey)
	if err != nil {
		log.Errorf("MailboxClient->SubscribeUser: InitPubsub error: %v", err)
		return err
	}

	user := &dmsgUser.LightMailboxUser{
		Target:        *target,
		ServicePeerID: "",
	}

	err = d.handlePubsubProtocol(&user.Target)
	if err != nil {
		log.Errorf("MailboxClient->SubscribeUser: handlePubsubProtocol error: %v", err)
		err := user.Target.Close()
		if err != nil {
			log.Warnf("MailboxClient->SubscribeUser: Target.Close error: %v", err)
			return err
		}
		return err
	}
	d.lightMailboxUser = user
	log.Debugf("MailboxClient->SubscribeUser end")
	return nil
}

func (d *MailboxClient) UnSubscribeUser() error {
	log.Debugf("MailboxClient->unsubscribeUser begin")
	if d.lightMailboxUser == nil {
		log.Errorf("MailboxClient->unsubscribeUser: userPubkey is not exist in destUserInfoList")
		return fmt.Errorf("MailboxClient->unsubscribeUser: userPubkey is not exist in destUserInfoList")
	}
	d.lightMailboxUser.Close()
	d.lightMailboxUser = nil
	log.Debugf("MailboxClient->unsubscribeUser end")
	return nil
}

func (d *MailboxClient) SetProxy(pubkey string) error {
	if pubkey == "" {
		return fmt.Errorf("MailboxClient->SetProxy: pubkey is empty")
	}

	if d.GetProxyPubkey() != "" {
		return fmt.Errorf("MailboxClient->SetProxy: proxyPubkey is not empty")
	}
	err := d.subscribeProxyUser(pubkey)
	if err != nil {
		return err
	}
	d.BaseService.SetProxyPubkey(pubkey)
	return nil
}

func (d *MailboxClient) ClearProxyPubkey() error {
	proxyPubkey := d.GetProxyPubkey()
	if proxyPubkey != "" {
		err := d.unsubscribeProxyUser()
		if err != nil {
			return err
		}
		d.SetProxyPubkey("")
	}
	return nil
}

func (d *MailboxClient) subscribeProxyUser(pubkey string) error {
	log.Debugf("MailboxClient->subscribeProxyUser begin\npubkey: %s", pubkey)
	if d.proxyUser != nil {
		log.Errorf("MailboxClient->subscribeProxyUser: user isn't nil")
		return fmt.Errorf("MailboxClient->subscribeProxyUser: user isn't nil")
	}

	target, err := dmsgUser.NewTarget(pubkey, nil)
	if err != nil {
		log.Errorf("MailboxClient->subscribeProxyUser: NewUser error: %v", err)
		return err
	}

	err = target.InitPubsub(pubkey)
	if err != nil {
		log.Errorf("MailboxClient->subscribeProxyUser: InitPubsub error: %v", err)
		return err
	}

	user := &dmsgUser.LightMailboxUser{
		Target:        *target,
		ServicePeerID: "",
	}

	d.proxyUser = user
	log.Debugf("MailboxClient->subscribeProxyUser end")
	return nil
}

func (d *MailboxClient) unsubscribeProxyUser() error {
	log.Debugf("MailboxClient->unsubscribeProxyUser begin")
	if d.proxyUser == nil {
		return fmt.Errorf("MailboxClient->unsubscribeProxyUser: userPubkey is not exist in destUserInfoList")
	}
	d.proxyUser.Close()
	d.proxyUser = nil
	log.Debugf("MailboxClient->unsubscribeProxyUser end")
	return nil
}

func (d *MailboxClient) IsExistMailbox(userPubkey string, timeout time.Duration) (*pb.SeekMailboxRes, error) {
	_, seekMailboxDoneChan, err := d.seekMailboxProtocol.Request(d.lightMailboxUser.Key.PubkeyHex, userPubkey)
	if err != nil {
		return nil, fmt.Errorf("MailboxClient->IsExistMailbox: seekMailboxProtocol.Request error : %+v", err)
	}

	select {
	case seekMailboxResponseProtoData := <-seekMailboxDoneChan:
		response, ok := seekMailboxResponseProtoData.(*pb.SeekMailboxRes)
		if !ok || response == nil {
			return response, fmt.Errorf("MailboxClient->IsExistMailbox: seekMailboxProtoData is not SeekMailboxRes")
		}
		if response.RetCode.Code < 0 {
			return response, fmt.Errorf("MailboxClient->IsExistMailbox: seekMailboxProtoData retcode.code < 0")
		} else {
			log.Debugf("MailboxClient->IsExistMailbox: seekMailboxProtoData success")
			return response, nil
		}
	case <-time.After(timeout):
		return nil, fmt.Errorf("MailboxClient->IsExistMailbox: time.After 3s timeout")
	case <-d.BaseService.TvBase.GetCtx().Done():
		log.Debug("MailboxClient->IsExistMailbox: BaseService.GetCtx().Done()")
	}
	return nil, fmt.Errorf("MailboxClient->IsExistMailbox: unknow error")
}

func (d *MailboxClient) CreateMailbox(timeout time.Duration) error {
	log.Debug("MailboxClient->CreateMailbox begin")

	pubkey := d.lightMailboxUser.Key.PubkeyHex
	if d.GetProxyPubkey() != "" {
		pubkey = d.GetProxyPubkey()
	}
	curtime := time.Now().UnixNano()
	resp, err := d.IsExistMailbox(pubkey, timeout)
	if err != nil {
		return err
	}
	remainTimeDuration := timeout - time.Duration(curtime)
	if remainTimeDuration >= 0 {
		switch resp.RetCode.Code {
		case dmsgProtocol.SuccCode:
			err = d.releaseUnusedMailbox(resp.BasicData.PeerID, pubkey, 30*time.Second)
			if err != nil {
				return err
			}
		case dmsgProtocol.NoExistCode:
			return d.createMailbox(pubkey, remainTimeDuration)
		}
	} else {
		return fmt.Errorf("MailboxClient->CreateMailboxWithProxy: timeout")
	}
	return nil
}

func (d *MailboxClient) createMailbox(pubkey string, timeout time.Duration) error {
	hostId := d.TvBase.GetHost().ID().String()
	servicePeerList, err := d.TvBase.GetAvailableServicePeerList(hostId)
	if err != nil {
		log.Errorf("MailboxClient->createMailbox: getAvailableServicePeerList error: %v", err)
		return err
	}

	peerID := d.TvBase.GetHost().ID().String()
	for _, servicePeerID := range servicePeerList {
		log.Debugf("MailboxClient->createMailbox: servicePeerID: %v", servicePeerID)
		if peerID == servicePeerID.String() {
			continue
		}
		_, createMailboxResponseChan, err := d.createMailboxProtocol.Request(servicePeerID, pubkey)
		if err != nil {
			log.Errorf("MailboxClient->createMailbox: createMailboxProtocol.Request error: %v", err)
			continue
		}

		select {
		case createMailboxResponseProtoData := <-createMailboxResponseChan:
			log.Debugf("MailboxClient->createMailbox: createMailboxResponseProtoData: %+v", createMailboxResponseProtoData)
			response, ok := createMailboxResponseProtoData.(*pb.CreateMailboxRes)
			if !ok || response == nil {
				log.Errorf("MailboxClient->createMailbox: createMailboxResponseChan is not CreateMailboxRes")
				continue
			}

			switch response.RetCode.Code {
			case 0, 1:
				log.Debugf("MailboxClient->createMailbox: createMailboxProtocol success")
				return nil
			default:
				continue
			}
		case <-time.After(timeout):
			continue
		case <-d.TvBase.GetCtx().Done():
			return nil
		}
	}
	log.Error("MailboxClient->createMailbox: no available service peers")
	return nil
}

func (d *MailboxClient) readMailbox(peerIdHex string, reqPubkey string, timeout time.Duration, clearMode bool) ([]msg.ReceiveMsg, error) {
	var msgList []msg.ReceiveMsg
	peerID, err := peer.Decode(peerIdHex)
	if err != nil {
		log.Errorf("MailboxClient->readMailbox: fail to decode peer id: %v", err)
		return msgList, err
	}

	for {
		_, readMailboxResponseChan, err := d.readMailboxMsgPrtocol.Request(peerID, reqPubkey, clearMode)
		if err != nil {
			return msgList, err
		}
		select {
		case responseProtoData := <-readMailboxResponseChan:
			response, ok := responseProtoData.(*pb.ReadMailboxRes)
			if !ok || response == nil || response.RetCode == nil {
				return msgList, fmt.Errorf("MailboxClient->readMailbox: response:%+v", response)
			}
			if response.RetCode.Code < 0 {
				return msgList, fmt.Errorf("MailboxClient->readMailbox: readMailboxRes fail")
			}
			receiveMsglist, err := d.parseReadMailboxResponse(response, msg.MsgDirection.From)
			if err != nil {
				return msgList, err
			}
			msgList = append(msgList, receiveMsglist...)
			if !response.ExistData {
				log.Debugf("MailboxClient->readMailbox: readMailboxChanResponseChan success")
				return msgList, nil
			}
			continue
		case <-time.After(timeout):
			return msgList, fmt.Errorf("MailboxClient->readMailbox: readMailboxResponseChan time out")
		case <-d.TvBase.GetCtx().Done():
			return msgList, fmt.Errorf("MailboxClient->readMailbox: BaseService.GetCtx().Done()")
		}
	}
}

func (d *MailboxClient) releaseUnusedMailbox(peerIdHex string, reqPubkey string, timeout time.Duration) error {
	log.Debug("MailboxClient->releaseUnusedMailbox begin")

	_, err := d.readMailbox(peerIdHex, reqPubkey, timeout, false)
	if err != nil {
		return err
	}

	if d.lightMailboxUser.ServicePeerID == "" {
		d.lightMailboxUser.ServicePeerID = peerIdHex
	} else if peerIdHex != d.lightMailboxUser.ServicePeerID {
		peerID, err := peer.Decode(peerIdHex)
		if err != nil {
			log.Errorf("MailboxClient->releaseUnusedMailbox: fail to decode peer id: %v", err)
			return err
		}
		_, releaseMailboxResponseChan, err := d.releaseMailboxPrtocol.Request(peerID, reqPubkey)
		if err != nil {
			return err
		}
		select {
		case <-releaseMailboxResponseChan:
			log.Debugf("MailboxClient->releaseUnusedMailbox: releaseMailboxResponseChan success")
		case <-time.After(time.Second * timeout):
			return fmt.Errorf("MailboxClient->releaseUnusedMailbox: releaseMailboxResponseChan time out")
		case <-d.TvBase.GetCtx().Done():
			return fmt.Errorf("MailboxClient->releaseUnusedMailbox: BaseService.GetCtx().Done()")
		}
	}

	log.Debugf("MailboxClient->releaseUnusedMailbox end")
	return nil
}

func (d *MailboxClient) parseReadMailboxResponse(responseProtoData protoreflect.ProtoMessage, direction string) ([]msg.ReceiveMsg, error) {
	log.Debugf("MailboxClient->parseReadMailboxResponse begin:\nresponseProtoData: %v", responseProtoData)
	msgList := []msg.ReceiveMsg{}
	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		log.Errorf("MailboxClient->parseReadMailboxResponse: fail to convert to *pb.ReadMailboxMsgRes")
		return msgList, fmt.Errorf("MailboxClient->parseReadMailboxResponse: fail to convert to *pb.ReadMailboxMsgRes")
	}

	for _, mailboxItem := range response.ContentList {
		log.Debugf("MailboxClient->parseReadMailboxResponse: msg key = %s", mailboxItem.Key)
		msgContent := mailboxItem.Content

		fields := strings.Split(mailboxItem.Key, msg.MsgKeyDelimiter)
		if len(fields) < msg.MsgFieldsLen {
			log.Errorf("MailboxClient->parseReadMailboxResponse: msg key fields len not enough")
			return msgList, fmt.Errorf("MailboxClient->parseReadMailboxResponse: msg key fields len not enough")
		}

		timeStamp, err := strconv.ParseInt(fields[msg.MsgTimeStampIndex], 10, 64)
		if err != nil {
			log.Errorf("MailboxClient->parseReadMailboxResponse: msg timeStamp parse error : %v", err)
			return msgList, fmt.Errorf("MailboxClient->parseReadMailboxResponse: msg timeStamp parse error : %v", err)
		}

		destPubkey := fields[msg.MsgSrcUserPubKeyIndex]
		srcPubkey := fields[msg.MsgDestUserPubKeyIndex]
		msgID := fields[msg.MsgIDIndex]

		msgList = append(msgList, msg.ReceiveMsg{
			ID:         msgID,
			ReqPubkey:  srcPubkey,
			DestPubkey: destPubkey,
			Content:    msgContent,
			TimeStamp:  timeStamp,
			Direction:  direction,
		})
	}
	log.Debug("MailboxClient->parseReadMailboxResponse end")
	return msgList, nil
}

func (d *MailboxClient) handlePubsubProtocol(target *dmsgUser.Target) error {
	ctx := d.TvBase.GetCtx()
	protocolDataChan, err := dmsgServiceCommon.WaitMessage(ctx, target.Key.PubkeyHex)
	if err != nil {
		return err
	}
	log.Debugf("MailboxClient->handlePubsubProtocol: protocolDataChan: %+v", protocolDataChan)
	go func() {
		for {
			select {
			case protocolHandle, ok := <-protocolDataChan:
				if !ok {
					return
				}
				pid := protocolHandle.PID
				log.Debugf("MailboxClient->handlePubsubProtocol: \npid: %d\ntopicName: %s", pid, target.Pubsub.Topic.String())

				handle := d.ProtocolHandleList[pid]
				if handle == nil {
					log.Debugf("MailboxClient->handlePubsubProtocol: no handle for pid: %d", pid)
					continue
				}
				msgRequestPID := d.pubsubMsgProtocol.Adapter.GetRequestPID()
				msgResponsePID := d.pubsubMsgProtocol.Adapter.GetResponsePID()
				seekRequestPID := d.seekMailboxProtocol.Adapter.GetRequestPID()
				seekResponsePID := d.seekMailboxProtocol.Adapter.GetResponsePID()
				data := protocolHandle.Data
				switch pid {
				case msgRequestPID:
					err = handle.HandleRequestData(data)
					if err != nil {
						log.Warnf("MailboxClient->handlePubsubProtocol: HandleRequestData error: %v", err)
					}
					continue
				case msgResponsePID:
					continue
				case seekRequestPID:
					err = handle.HandleRequestData(data)
					if err != nil {
						log.Warnf("MailboxClient->handlePubsubProtocol: HandleRequestData error: %v", err)
					}
					continue
				case seekResponsePID:
					err = handle.HandleResponseData(data)
					if err != nil {
						log.Warnf("MailboxClient->handlePubsubProtocol: HandleResponseData error: %v", err)
					}
					continue
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}
