package mailbox

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/libp2p/go-libp2p/core/peer"
	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/common/db"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	dmsgCommonUtil "github.com/tinyverse-web3/tvbase/dmsg/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	dmsgCommonService "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	tvutilKey "github.com/tinyverse-web3/tvutil/key"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MailboxService struct {
	dmsgCommonService.BaseService
	createMailboxProtocol *dmsgProtocol.MailboxSProtocol
	releaseMailboxPrtocol *dmsgProtocol.MailboxSProtocol
	readMailboxMsgPrtocol *dmsgProtocol.MailboxSProtocol
	seekMailboxProtocol   *dmsgProtocol.MailboxPProtocol
	sendMsgProtocol       *dmsgProtocol.MsgPProtocol
	lightMailboxUser      *dmsgUser.LightMailboxUser
	onReceiveMsg          msg.OnReceiveMsg
	serviceUserList       map[string]*dmsgUser.ServiceMailboxUser
	datastore             db.Datastore
	stopCleanRestResource chan bool
}

func CreateService(tvbaseService tvbaseCommon.TvBaseService) (*MailboxService, error) {
	d := &MailboxService{}
	err := d.Init(tvbaseService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *MailboxService) Init(tvbaseService tvbaseCommon.TvBaseService) error {
	err := d.BaseService.Init(tvbaseService)
	if err != nil {
		return err
	}
	d.serviceUserList = make(map[string]*dmsgUser.ServiceMailboxUser)
	return nil
}

// sdk-common
func (d *MailboxService) Start(
	enableService bool,
	pubkeyData []byte,
	getSig dmsgKey.GetSigCallback) error {
	log.Logger.Debug("DmsgService->Start begin")
	if d.EnableService {
		var err error
		d.datastore, err = db.CreateBadgerDB(d.GetConfig().DatastorePath)
		if err != nil {
			log.Logger.Errorf("dmsgService->Start: create datastore error %v", err)
			return err
		}
		d.stopCleanRestResource = make(chan bool)
		d.cleanRestResource()
	}

	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()
	// stream protocol
	d.createMailboxProtocol = adapter.NewCreateMailboxProtocol(ctx, host, d, d)
	d.releaseMailboxPrtocol = adapter.NewReleaseMailboxProtocol(ctx, host, d, d)
	d.readMailboxMsgPrtocol = adapter.NewReadMailboxMsgProtocol(ctx, host, d, d)

	// pubsub protocol
	d.seekMailboxProtocol = adapter.NewSeekMailboxProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(d.seekMailboxProtocol.Adapter.GetRequestPID(), d.seekMailboxProtocol)
	d.sendMsgProtocol = adapter.NewSendMsgProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(d.sendMsgProtocol.Adapter.GetRequestPID(), d.sendMsgProtocol)

	// user
	d.initUser(pubkeyData, getSig)
	log.Logger.Debug("DmsgService->Start end")
	return nil
}

func (d *MailboxService) Stop() error {
	log.Logger.Debug("DmsgService->Stop begin")
	d.UnregistPubsubProtocol(d.seekMailboxProtocol.Adapter.GetRequestPID())
	d.unsubscribeUser()
	d.unsubscribeServiceUserList()

	if d.datastore != nil {
		d.datastore.Close()
		d.datastore = nil
	}
	d.stopCleanRestResource <- true
	close(d.stopCleanRestResource)
	log.Logger.Debug("DmsgService->Stop end")
	return nil
}

func (d *MailboxService) SetOnReceiveMsg(cb msg.OnReceiveMsg) {
	d.onReceiveMsg = cb
}

// sdk-msg
func (d *MailboxService) RequestReadMailbox(timeout time.Duration) ([]msg.Msg, error) {
	log.Logger.Debugf("DmsgService->RequestReadMailbox begin\ntimeout: %+v", timeout)
	var msgList []msg.Msg
	peerID, err := peer.Decode(d.lightMailboxUser.ServicePeerID)
	if err != nil {
		log.Logger.Errorf("DmsgService->RequestReadMailbox: peer.Decode error: %v", err)
		return msgList, err
	}
	_, readMailboxDoneChan, err := d.readMailboxMsgPrtocol.Request(peerID, d.lightMailboxUser.Key.PubkeyHex)
	if err != nil {
		return msgList, err
	}

	select {
	case responseProtoData := <-readMailboxDoneChan:
		log.Logger.Debugf("DmsgService->RequestReadMailbox: responseProtoData: %+v", responseProtoData)
		response, ok := responseProtoData.(*pb.ReadMailboxRes)
		if !ok || response == nil {
			log.Logger.Errorf("DmsgService->RequestReadMailbox: readMailboxDoneChan is not ReadMailboxRes")
			return msgList, fmt.Errorf("DmsgService->RequestReadMailbox: readMailboxDoneChan is not ReadMailboxRes")
		}
		log.Logger.Debugf("DmsgService->RequestReadMailbox: readMailboxChanDoneChan success")
		msgList, err = d.parseReadMailboxResponse(response, msg.MsgDirection.From)
		if err != nil {
			return msgList, err
		}

		return msgList, nil
	case <-time.After(timeout):
		log.Logger.Debugf("DmsgService->RequestReadMailbox: timeout")
		return msgList, fmt.Errorf("DmsgService->RequestReadMailbox: timeout")
	case <-d.TvBase.GetCtx().Done():
		log.Logger.Debugf("DmsgService->RequestReadMailbox: BaseService.GetCtx().Done()")
		return msgList, fmt.Errorf("DmsgService->RequestReadMailbox: BaseService.GetCtx().Done()")
	}
}

// DmsgServiceInterface
func (d *MailboxService) GetUserPubkeyHex() (string, error) {
	if d.lightMailboxUser == nil {
		return "", fmt.Errorf("DmsgService->GetUserPubkeyHex: user is nil")
	}
	return d.lightMailboxUser.Key.PubkeyHex, nil
}

func (d *MailboxService) GetUserSig(protoData []byte) ([]byte, error) {
	if d.lightMailboxUser == nil {
		log.Logger.Errorf("DmsgService->GetUserSig: user is nil")
		return nil, fmt.Errorf("DmsgService->GetUserSig: user is nil")
	}
	return d.lightMailboxUser.GetSig(protoData)
}

func (d *MailboxService) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	user := d.serviceUserList[pubkey]
	if user == nil {
		log.Logger.Errorf("DmsgService->GetPublishTarget: service user is nil")
		return nil, fmt.Errorf("DmsgService->GetPublishTarget: service user is nil")
	}
	return &user.Target, nil
}

// MailboxSpCallback
func (d *MailboxService) OnCreateMailboxRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	log.Logger.Debugf("dmsgService->OnCreateMailboxRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		log.Logger.Errorf("dmsgService->OnCreateMailboxRequest: fail to convert requestProtoData to *pb.CreateMailboxReq")
		return nil, nil, fmt.Errorf("dmsgService->OnCreateMailboxRequest: fail to convert requestProtoData to *pb.CreateMailboxReq")
	}
	isAvailable := d.isAvailableMailbox(request.BasicData.Pubkey)
	if !isAvailable {
		log.Logger.Errorf("dmsgService->OnCreateMailboxRequest: exceeded the maximum number of mailbox service")
		return nil, nil, errors.New("dmsgService->OnCreateMailboxRequest: exceeded the maximum number of mailbox service")
	}
	user := d.getServiceUser(request.BasicData.Pubkey)
	if user != nil {
		log.Logger.Errorf("dmsgService->OnCreateMailboxRequest: user public key pubsub already exist")
		retCode := &pb.RetCode{
			Code:   dmsgProtocol.AlreadyExistCode,
			Result: "dmsgService->OnCreateMailboxRequest: user public key pubsub already exist",
		}
		return nil, retCode, nil
	}

	err := d.subscribeServiceUser(request.BasicData.Pubkey)
	if err != nil {
		return nil, nil, err
	}
	log.Logger.Debugf("dmsgService->OnCreateMailboxRequest end")
	return nil, nil, nil
}

func (d *MailboxService) OnCreateMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Logger.Debug("DmsgService->OnCreateMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	request, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		log.Logger.Errorf("DmsgService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.CreateMailboxReq")
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.CreateMailboxReq")
	}
	response, ok := responseProtoData.(*pb.CreateMailboxRes)
	if !ok {
		log.Logger.Errorf("DmsgService->OnCreateMailboxResponse: fail to convert responseProtoData to *pb.CreateMailboxRes")
		return nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: fail to convert responseProtoData to *pb.CreateMailboxRes")
	}

	switch response.RetCode.Code {
	case 0: // new
		fallthrough
	case 1: // exist mailbox
		log.Logger.Debug("DmsgService->OnCreateMailboxResponse: mailbox has created, read message from mailbox...")
		err := d.releaseUnusedMailbox(response.BasicData.PeerID, request.BasicData.Pubkey)
		if err != nil {
			return nil, err
		}
	case -1:
		log.Logger.Warnf("DmsgService->OnCreateMailboxResponse: fail RetCode: %+v ", response.RetCode)
	default:
		log.Logger.Warnf("DmsgService->OnCreateMailboxResponse: other case RetCode: %+v", response.RetCode)
	}

	log.Logger.Debug("DmsgService->OnCreateMailboxResponse end")
	return nil, nil
}

func (d *MailboxService) OnReleaseMailboxRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	log.Logger.Debugf("dmsgService->OnReleaseMailboxRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.ReleaseMailboxReq)
	if !ok {
		log.Logger.Errorf("dmsgService->OnReleaseMailboxRequest: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
		return nil, nil, fmt.Errorf("dmsgService->OnReleaseMailboxRequest: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
	}
	err := d.unsubscribeServiceUser(request.BasicData.Pubkey)
	if err != nil {
		return nil, nil, err
	}
	log.Logger.Debugf("dmsgService->OnReleaseMailboxRequest end")
	return nil, nil, nil
}

func (d *MailboxService) OnReleaseMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Logger.Debug(
		"DmsgService->OnReleaseMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	_, ok := requestProtoData.(*pb.ReleaseMailboxReq)
	if !ok {
		log.Logger.Errorf("DmsgService->OnReleaseMailboxResponse: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
		return nil, fmt.Errorf("DmsgService->OnReleaseMailboxResponse: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
	}

	response, ok := responseProtoData.(*pb.ReleaseMailboxRes)
	if !ok {
		log.Logger.Errorf("DmsgService->OnReleaseMailboxResponse: fail to convert responseProtoData to *pb.ReleaseMailboxRes")
		return nil, fmt.Errorf("DmsgService->OnReleaseMailboxResponse: fail to convert responseProtoData to *pb.ReleaseMailboxRes")
	}
	if response.RetCode.Code < 0 {
		log.Logger.Warnf("DmsgService->OnReleaseMailboxResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("DmsgService->OnReleaseMailboxResponse: fail RetCode: %+v", response.RetCode)
	}

	log.Logger.Debug("DmsgService->OnReleaseMailboxResponse end")
	return nil, nil
}

func (d *MailboxService) OnReadMailboxMsgRequest(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	log.Logger.Debugf("dmsgService->OnReadMailboxMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.ReadMailboxReq)
	if !ok {
		log.Logger.Errorf("dmsgService->OnReadMailboxMsgRequest: fail to convert requestProtoData to *pb.ReadMailboxReq")
		return nil, nil, fmt.Errorf("dmsgService->OnReadMailboxMsgRequest: fail to convert requestProtoData to *pb.ReadMailboxReq")
	}

	pubkey := request.BasicData.Pubkey
	user := d.getServiceUser(pubkey)
	if user == nil {
		log.Logger.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot find user for pubkey: %s", pubkey)
		return nil, nil, fmt.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot find user for pubkey: %s", pubkey)
	}
	user.MsgRWMutex.RLock()
	defer user.MsgRWMutex.RUnlock()

	var query = query.Query{
		Prefix: d.getMsgPrefix(pubkey),
	}
	results, err := d.datastore.Query(d.TvBase.GetCtx(), query)
	if err != nil {
		return nil, nil, err
	}
	defer results.Close()

	mailboxMsgDataList := []*pb.MailboxItem{}
	find := false
	needDeleteKeyList := []string{}
	for result := range results.Next() {
		mailboxMsgData := &pb.MailboxItem{
			Key:     string(result.Key),
			Content: result.Value,
		}
		mailboxMsgDataList = append(mailboxMsgDataList, mailboxMsgData)
		needDeleteKeyList = append(needDeleteKeyList, string(result.Key))
		find = true
	}

	for _, needDeleteKey := range needDeleteKeyList {
		err := d.datastore.Delete(d.TvBase.GetCtx(), datastore.NewKey(needDeleteKey))
		if err != nil {
			log.Logger.Errorf("dmsgService->OnReadMailboxMsgRequest: datastore.Delete error: %+v", err)
		}
	}

	if !find {
		log.Logger.Debug("dmsgService->OnReadMailboxMsgRequest: user msgs is empty")
	}
	log.Logger.Debugf("dmsgService->OnReadMailboxMsgRequest end")
	return mailboxMsgDataList, nil, nil
}

func (d *MailboxService) OnReadMailboxMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Logger.Debug("DmsgService->OnReadMailboxMsgResponse: begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		log.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: fail to convert requestProtoData to *pb.ReadMailboxMsgRes")
		return nil, fmt.Errorf("DmsgService->OnReadMailboxMsgResponse: fail to convert requestProtoData to *pb.ReadMailboxMsgRes")
	}

	log.Logger.Debugf("DmsgService->OnReadMailboxMsgResponse: found (%d) new message", len(response.ContentList))

	msgList, err := d.parseReadMailboxResponse(responseProtoData, msg.MsgDirection.From)
	if err != nil {
		return nil, err
	}
	for _, msg := range msgList {
		log.Logger.Debugf("DmsgService->OnReadMailboxMsgResponse: From = %s, To = %s", msg.SrcPubkey, msg.DestPubkey)
		if d.onReceiveMsg != nil {
			d.onReceiveMsg(msg.SrcPubkey, msg.DestPubkey, msg.Content, msg.TimeStamp, msg.ID, msg.Direction)
		} else {
			log.Logger.Errorf("DmsgService->OnReadMailboxMsgResponse: OnReceiveMsg is nil")
		}
	}

	log.Logger.Debug("DmsgService->OnReadMailboxMsgResponse end")
	return nil, nil
}

// MailboxPpCallback
func (d *MailboxService) OnSeekMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	log.Logger.Debug("DmsgService->OnSeekMailboxRequest begin\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.SeekMailboxReq)
	if !ok {
		log.Logger.Errorf("DmsgService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.SeekMailboxReq")
		return nil, nil, fmt.Errorf("DmsgService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.SeekMailboxReq")
	}

	if request.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		log.Logger.Debugf("dmsgService->OnReleaseMailboxRequest: request.BasicData.PeerID == d.BaseService.GetHost().ID")
		return nil, nil, fmt.Errorf("dmsgService->OnReleaseMailboxRequest: request.BasicData.PeerID == d.BaseService.GetHost().ID")
	}

	log.Logger.Debug("DmsgService->OnSeekMailboxRequest end")
	return nil, nil, nil
}

func (d *MailboxService) OnSeekMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Logger.Debug(
		"DmsgService->OnSeekMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	log.Logger.Debug("DmsgService->OnSeekMailboxResponse end")
	return nil, nil
}

func (d *MailboxService) OnSendMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, error) {
	log.Logger.Debugf("MailboxService->OnSendMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		log.Logger.Errorf("MailboxService->OnSendMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
		return nil, nil, fmt.Errorf("MailboxService->OnSendMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
	}

	if d.EnableService {
		pubkey := request.BasicData.Pubkey
		user := d.getServiceUser(pubkey)
		if user == nil {
			log.Logger.Errorf("MailboxService->OnSendMsgRequest: public key %s is not exist", pubkey)
			return nil, nil, fmt.Errorf("MailboxService->OnSendMsgRequest: public key %s is not exist", pubkey)
		}
		user.LastReciveTimestamp = time.Now().UnixNano()
		d.saveUserMsg(requestProtoData)
	}

	log.Logger.Debugf("MailboxService->OnSendMsgRequest end")
	return nil, nil, nil
}

func (d *MailboxService) OnSendMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Logger.Debugf(
		"dmsgService->OnSendMsgResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	response, ok := responseProtoData.(*pb.SendMsgRes)
	if !ok {
		log.Logger.Errorf("DmsgService->OnSendMsgResponse: cannot convert %v to *pb.SendMsgRes", responseProtoData)
		return nil, fmt.Errorf("DmsgService->OnSendMsgResponse: cannot convert %v to *pb.SendMsgRes", responseProtoData)
	}
	if response.RetCode.Code != 0 {
		log.Logger.Warnf("DmsgService->OnSendMsgResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("DmsgService->OnSendMsgResponse: fail RetCode: %+v", response.RetCode)
	}
	log.Logger.Debugf("dmsgService->OnSendMsgResponse end")
	return nil, nil
}

// common
func (d *MailboxService) cleanRestResource() {
	go func() {
		ticker := time.NewTicker(3 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-d.stopCleanRestResource:
				return
			case <-ticker.C:
				for pubkey, pubsub := range d.serviceUserList {
					days := dmsgCommonUtil.DaysBetween(pubsub.LastReciveTimestamp, time.Now().UnixNano())
					// delete mailbox msg in datastore and unsubscribe mailbox when days is over, default days is 30
					if days >= d.GetConfig().KeepMailboxMsgDay {
						var query = query.Query{
							Prefix:   d.getMsgPrefix(pubkey),
							KeysOnly: true,
						}
						results, err := d.datastore.Query(d.TvBase.GetCtx(), query)
						if err != nil {
							log.Logger.Errorf("dmsgService->readDestUserPubsub: query error: %v", err)
						}

						for result := range results.Next() {
							d.datastore.Delete(d.TvBase.GetCtx(), datastore.NewKey(result.Key))
							log.Logger.Debugf("dmsgService->readDestUserPubsub: delete msg by key:%v", string(result.Key))
						}

						d.unsubscribeServiceUser(pubkey)
						return
					}
				}
				continue
			case <-d.TvBase.GetCtx().Done():
				return
			}
		}
	}()
}

func (d *MailboxService) handlePubsubProtocol(target *dmsgUser.Target) {
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

// user
func (d *MailboxService) initUser(pubkeyData []byte, getSig dmsgKey.GetSigCallback) error {
	log.Logger.Debug("DmsgService->initUser begin")
	pubkey := tvutilKey.TranslateKeyProtoBufToString(pubkeyData)
	err := d.subscribeUser(pubkey, getSig)
	if err != nil {
		return err
	}

	if !d.EnableService {
		select {
		case <-d.TvBase.GetRendezvousChan():
			d.initMailbox(pubkey)
		case <-time.After(1 * time.Hour):
			return nil
		case <-d.TvBase.GetCtx().Done():
			return d.TvBase.GetCtx().Err()
		}
	}

	log.Logger.Debug("DmsgService->initUser end")
	return nil
}

func (d *MailboxService) subscribeUser(pubkey string, getSig dmsgKey.GetSigCallback) error {
	log.Logger.Debugf("DmsgService->subscribeUser begin\npubkey: %s", pubkey)
	if d.lightMailboxUser != nil {
		log.Logger.Errorf("DmsgService->subscribeUser: user isn't nil")
		return fmt.Errorf("DmsgService->subscribeUser: user isn't nil")
	}

	if d.serviceUserList[pubkey] != nil {
		log.Logger.Errorf("DmsgService->subscribeUser: pubkey is already exist in serviceUserList")
		return fmt.Errorf("DmsgService->subscribeUser: pubkey is already exist in serviceUserList")
	}

	target, err := dmsgUser.NewTarget(d.TvBase.GetCtx(), pubkey, getSig)
	if err != nil {
		log.Logger.Errorf("DmsgService->subscribeUser: NewUser error: %v", err)
		return err
	}

	err = target.InitPubsub(d.Pubsub, pubkey)
	if err != nil {
		log.Logger.Errorf("DmsgService->subscribeUser: InitPubsub error: %v", err)
		return err
	}

	d.lightMailboxUser = &dmsgUser.LightMailboxUser{
		Target:        *target,
		ServicePeerID: "",
	}

	log.Logger.Debugf("DmsgService->subscribeUser end")
	return nil
}

func (d *MailboxService) unsubscribeUser() error {
	log.Logger.Debugf("DmsgService->unsubscribeUser begin")
	if d.lightMailboxUser == nil {
		log.Logger.Errorf("DmsgService->unsubscribeUser: userPubkey is not exist in destUserInfoList")
		return fmt.Errorf("DmsgService->unsubscribeUser: userPubkey is not exist in destUserInfoList")
	}
	d.lightMailboxUser.Close()
	d.lightMailboxUser = nil
	log.Logger.Debugf("DmsgService->unsubscribeUser end")
	return nil
}

// dest user
func (d *MailboxService) getServiceUser(pubkey string) *dmsgUser.ServiceMailboxUser {
	return d.serviceUserList[pubkey]
}

func (d *MailboxService) subscribeServiceUser(pubkey string) error {
	log.Logger.Debug("DmsgService->subscribeServiceUser begin\npubkey: %s", pubkey)
	if d.serviceUserList[pubkey] != nil {
		log.Logger.Errorf("DmsgService->subscribeServiceUser: pubkey is already exist in serviceUserList")
		return fmt.Errorf("DmsgService->subscribeServiceUser: pubkey is already exist in serviceUserList")
	}
	target, err := dmsgUser.NewTarget(d.TvBase.GetCtx(), pubkey, nil)
	if err != nil {
		log.Logger.Errorf("DmsgService->subscribeServiceUser: NewTarget error: %v", err)
		return err
	}
	err = target.InitPubsub(d.Pubsub, pubkey)
	if err != nil {
		log.Logger.Errorf("DmsgService->subscribeServiceUser: InitPubsub error: %v", err)
		return err
	}
	user := &dmsgUser.ServiceMailboxUser{
		DestTarget: dmsgUser.DestTarget{
			Target:              *target,
			LastReciveTimestamp: time.Now().UnixNano(),
		},
		MsgRWMutex: sync.RWMutex{},
	}

	d.serviceUserList[pubkey] = user
	// go d.BaseService.DiscoverRendezvousPeers()
	go d.handlePubsubProtocol(&user.Target)
	return nil
}

func (d *MailboxService) unsubscribeServiceUser(pubkey string) error {
	log.Logger.Debugf("DmsgService->unsubscribeServiceUser begin\npubkey: %s", pubkey)

	user := d.serviceUserList[pubkey]
	if user == nil {
		log.Logger.Errorf("DmsgService->unsubscribeServiceUser: pubkey is not exist in serviceUserList")
		return fmt.Errorf("DmsgService->unsubscribeServiceUser: pubkey is not exist in serviceUserList")
	}
	user.Close()
	delete(d.serviceUserList, pubkey)

	log.Logger.Debug("DmsgService->unsubscribeServiceUser end")
	return nil
}

func (d *MailboxService) unsubscribeServiceUserList() error {
	for pubKey := range d.serviceUserList {
		d.unsubscribeServiceUser(pubKey)
	}
	return nil
}

// mailbox
func (d *MailboxService) isAvailableMailbox(pubKey string) bool {
	destUserCount := len(d.serviceUserList)
	return destUserCount < d.GetConfig().MaxMailboxPubsubCount
}

func (d *MailboxService) initMailbox(pubkey string) {
	log.Logger.Debug("DmsgService->initMailbox begin")
	_, seekMailboxDoneChan, err := d.seekMailboxProtocol.Request(d.lightMailboxUser.Key.PubkeyHex, d.lightMailboxUser.Key.PubkeyHex)
	if err != nil {
		return
	}
	select {
	case seekMailboxResponseProtoData := <-seekMailboxDoneChan:
		log.Logger.Debugf("DmsgService->initMailbox: seekMailboxProtoData: %+v", seekMailboxResponseProtoData)
		response, ok := seekMailboxResponseProtoData.(*pb.SeekMailboxRes)
		if !ok || response == nil {
			log.Logger.Errorf("DmsgService->initMailbox: seekMailboxProtoData is not SeekMailboxRes")
			// skip seek when seek mailbox quest fail (server err), create a new mailbox
		}
		if response.RetCode.Code < 0 {
			log.Logger.Errorf("DmsgService->initMailbox: seekMailboxProtoData fail")
			// skip seek when seek mailbox quest fail, create a new mailbox
		} else {
			log.Logger.Debugf("DmsgService->initMailbox: seekMailboxProtoData success")
			go d.releaseUnusedMailbox(response.BasicData.PeerID, pubkey)
			return
		}
	case <-time.After(3 * time.Second):
		log.Logger.Debugf("DmsgService->initMailbox: time.After 3s, create new mailbox")
		// begin create new mailbox

		hostId := d.TvBase.GetHost().ID().String()
		servicePeerList, err := d.TvBase.GetAvailableServicePeerList(hostId)
		if err != nil {
			log.Logger.Errorf("DmsgService->initMailbox: getAvailableServicePeerList error: %v", err)
			return
		}

		peerID := d.TvBase.GetHost().ID().String()
		for _, servicePeerID := range servicePeerList {
			log.Logger.Debugf("DmsgService->initMailbox: servicePeerID: %v", servicePeerID)
			if peerID == servicePeerID.String() {
				continue
			}
			_, createMailboxDoneChan, err := d.createMailboxProtocol.Request(servicePeerID, pubkey)
			if err != nil {
				log.Logger.Errorf("DmsgService->initMailbox: createMailboxProtocol.Request error: %v", err)
				continue
			}

			select {
			case createMailboxResponseProtoData := <-createMailboxDoneChan:
				log.Logger.Debugf("DmsgService->initMailbox: createMailboxResponseProtoData: %+v", createMailboxResponseProtoData)
				response, ok := createMailboxResponseProtoData.(*pb.CreateMailboxRes)
				if !ok || response == nil {
					log.Logger.Errorf("DmsgService->initMailbox: createMailboxDoneChan is not CreateMailboxRes")
					continue
				}

				switch response.RetCode.Code {
				case 0, 1:
					log.Logger.Debugf("DmsgService->initMailbox: createMailboxProtocol success")
					return
				default:
					continue
				}
			case <-time.After(time.Second * 3):
				continue
			case <-d.TvBase.GetCtx().Done():
				return
			}
		}

		log.Logger.Error("DmsgService->InitUser: no available service peers")
		return
		// end create mailbox
	case <-d.TvBase.GetCtx().Done():
		log.Logger.Debug("DmsgService->InitUser: BaseService.GetCtx().Done()")
		return
	}
}

func (d *MailboxService) releaseUnusedMailbox(peerIdHex string, pubkey string) error {
	log.Logger.Debug("DmsgService->releaseUnusedMailbox begin")

	peerID, err := peer.Decode(peerIdHex)
	if err != nil {
		log.Logger.Warnf("DmsgService->releaseUnusedMailbox: fail to decode peer id: %v", err)
		return err
	}
	_, readMailboxDoneChan, err := d.readMailboxMsgPrtocol.Request(peerID, pubkey)
	if err != nil {
		return err
	}

	select {
	case <-readMailboxDoneChan:
		if d.lightMailboxUser.ServicePeerID == "" {
			d.lightMailboxUser.ServicePeerID = peerIdHex
		} else if peerIdHex != d.lightMailboxUser.ServicePeerID {
			_, releaseMailboxDoneChan, err := d.releaseMailboxPrtocol.Request(peerID, pubkey)
			if err != nil {
				return err
			}
			select {
			case <-releaseMailboxDoneChan:
				log.Logger.Debugf("DmsgService->releaseUnusedMailbox: releaseMailboxDoneChan success")
			case <-time.After(time.Second * 3):
				return fmt.Errorf("DmsgService->releaseUnusedMailbox: releaseMailboxDoneChan time out")
			case <-d.TvBase.GetCtx().Done():
				return fmt.Errorf("DmsgService->releaseUnusedMailbox: BaseService.GetCtx().Done()")
			}
		}
	case <-time.After(time.Second * 10):
		return fmt.Errorf("DmsgService->releaseUnusedMailbox: readMailboxDoneChan time out")
	case <-d.TvBase.GetCtx().Done():
		return fmt.Errorf("DmsgService->releaseUnusedMailbox: BaseService.GetCtx().Done()")
	}
	log.Logger.Debugf("DmsgService->releaseUnusedMailbox end")
	return nil
}

func (d *MailboxService) parseReadMailboxResponse(responseProtoData protoreflect.ProtoMessage, direction string) ([]msg.Msg, error) {
	log.Logger.Debugf("DmsgService->parseReadMailboxResponse begin:\nresponseProtoData: %v", responseProtoData)
	msgList := []msg.Msg{}
	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		log.Logger.Errorf("DmsgService->parseReadMailboxResponse: fail to convert to *pb.ReadMailboxMsgRes")
		return msgList, fmt.Errorf("DmsgService->parseReadMailboxResponse: fail to convert to *pb.ReadMailboxMsgRes")
	}

	for _, mailboxItem := range response.ContentList {
		log.Logger.Debugf("DmsgService->parseReadMailboxResponse: msg key = %s", mailboxItem.Key)
		msgContent := mailboxItem.Content

		fields := strings.Split(mailboxItem.Key, msg.MsgKeyDelimiter)
		if len(fields) < msg.MsgFieldsLen {
			log.Logger.Errorf("DmsgService->parseReadMailboxResponse: msg key fields len not enough")
			return msgList, fmt.Errorf("DmsgService->parseReadMailboxResponse: msg key fields len not enough")
		}

		timeStamp, err := strconv.ParseInt(fields[msg.MsgTimeStampIndex], 10, 64)
		if err != nil {
			log.Logger.Errorf("DmsgService->parseReadMailboxResponse: msg timeStamp parse error : %v", err)
			return msgList, fmt.Errorf("DmsgService->parseReadMailboxResponse: msg timeStamp parse error : %v", err)
		}

		destPubkey := fields[msg.MsgSrcUserPubKeyIndex]
		srcPubkey := fields[msg.MsgDestUserPubKeyIndex]
		msgID := fields[msg.MsgIDIndex]

		msgList = append(msgList, msg.Msg{
			ID:         msgID,
			SrcPubkey:  srcPubkey,
			DestPubkey: destPubkey,
			Content:    msgContent,
			TimeStamp:  timeStamp,
			Direction:  direction,
		})
	}
	log.Logger.Debug("DmsgService->parseReadMailboxResponse end")
	return msgList, nil
}

func (d *MailboxService) saveUserMsg(protoMsg protoreflect.ProtoMessage) error {
	sendMsgReq, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		log.Logger.Errorf("dmsgService->saveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return fmt.Errorf("dmsgService->saveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}

	pubkey := sendMsgReq.BasicData.Pubkey
	user := d.getServiceUser(pubkey)
	if user == nil {
		log.Logger.Errorf("dmsgService->saveUserMsg: cannot find src user pubsub for %v", pubkey)
		return fmt.Errorf("dmsgService->saveUserMsg: cannot find src user pubsub for %v", pubkey)
	}

	user.MsgRWMutex.RLock()
	defer user.MsgRWMutex.RUnlock()

	key := d.getFullFromMsgPrefix(sendMsgReq)
	err := d.datastore.Put(d.TvBase.GetCtx(), datastore.NewKey(key), sendMsgReq.Content)
	if err != nil {
		return err
	}

	return err
}

func (d *MailboxService) getMsgPrefix(pubkey string) string {
	return msg.MsgPrefix + pubkey
}

func (d *MailboxService) getBasicFromMsgPrefix(srcUserPubkey string, destUserPubkey string) string {
	return msg.MsgPrefix + destUserPubkey + msg.MsgKeyDelimiter + srcUserPubkey
}

func (d *MailboxService) getFullFromMsgPrefix(sendMsgReq *pb.SendMsgReq) string {
	basicPrefix := d.getBasicFromMsgPrefix(sendMsgReq.BasicData.Pubkey, sendMsgReq.DestPubkey)
	direction := msg.MsgDirection.From
	return basicPrefix + msg.MsgKeyDelimiter +
		direction + msg.MsgKeyDelimiter +
		sendMsgReq.BasicData.ID + msg.MsgKeyDelimiter +
		strconv.FormatInt(sendMsgReq.BasicData.TS, 10)
}
