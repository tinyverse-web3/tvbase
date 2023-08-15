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
	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/common/db"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	dmsgCommonUtil "github.com/tinyverse-web3/tvbase/dmsg/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	tvutilKey "github.com/tinyverse-web3/tvutil/key"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var log = ipfsLog.Logger("dmsg.service.mailbox")

type MailboxService struct {
	dmsgServiceCommon.BaseService
	createMailboxProtocol *dmsgProtocol.MailboxSProtocol
	releaseMailboxPrtocol *dmsgProtocol.MailboxSProtocol
	readMailboxMsgPrtocol *dmsgProtocol.MailboxSProtocol
	seekMailboxProtocol   *dmsgProtocol.MailboxPProtocol
	pubsubMsgProtocol     *dmsgProtocol.PubsubMsgProtocol
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
	log.Debugf("MailboxService->Start begin\nenableService: %v", enableService)
	d.BaseService.Start(enableService)
	if d.EnableService {
		var err error
		d.datastore, err = db.CreateBadgerDB(d.GetConfig().DatastorePath)
		if err != nil {
			log.Errorf("dmsgService->Start: create datastore error %v", err)
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
	d.RegistPubsubProtocol(d.seekMailboxProtocol.Adapter.GetResponsePID(), d.seekMailboxProtocol)

	d.pubsubMsgProtocol = adapter.NewPubsubMsgProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetRequestPID(), d.pubsubMsgProtocol)
	d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetResponsePID(), d.pubsubMsgProtocol)
	// user
	d.initUser(pubkeyData, getSig)
	log.Debug("MailboxService->Start end")
	return nil
}

func (d *MailboxService) Stop() error {
	log.Debug("MailboxService->Stop begin")
	d.UnregistPubsubProtocol(d.seekMailboxProtocol.Adapter.GetRequestPID())
	d.unsubscribeUser()
	d.unsubscribeServiceUserList()

	if d.datastore != nil {
		d.datastore.Close()
		d.datastore = nil
	}
	d.stopCleanRestResource <- true
	close(d.stopCleanRestResource)
	log.Debug("MailboxService->Stop end")
	return nil
}

func (d *MailboxService) SetOnReceiveMsg(cb msg.OnReceiveMsg) {
	d.onReceiveMsg = cb
}

// sdk-msg
func (d *MailboxService) RequestReadMailbox(timeout time.Duration) ([]msg.Msg, error) {
	log.Debugf("MailboxService->RequestReadMailbox begin\ntimeout: %+v", timeout)
	var msgList []msg.Msg
	peerID, err := peer.Decode(d.lightMailboxUser.ServicePeerID)
	if err != nil {
		log.Errorf("MailboxService->RequestReadMailbox: peer.Decode error: %v", err)
		return msgList, err
	}
	_, readMailboxDoneChan, err := d.readMailboxMsgPrtocol.Request(peerID, d.lightMailboxUser.Key.PubkeyHex)
	if err != nil {
		return msgList, err
	}

	select {
	case responseProtoData := <-readMailboxDoneChan:
		log.Debugf("MailboxService->RequestReadMailbox: responseProtoData: %+v", responseProtoData)
		response, ok := responseProtoData.(*pb.ReadMailboxRes)
		if !ok || response == nil {
			log.Errorf("MailboxService->RequestReadMailbox: readMailboxDoneChan is not ReadMailboxRes")
			return msgList, fmt.Errorf("MailboxService->RequestReadMailbox: readMailboxDoneChan is not ReadMailboxRes")
		}
		log.Debugf("MailboxService->RequestReadMailbox: readMailboxChanDoneChan success")
		msgList, err = d.parseReadMailboxResponse(response, msg.MsgDirection.From)
		if err != nil {
			return msgList, err
		}

		return msgList, nil
	case <-time.After(timeout):
		log.Debugf("MailboxService->RequestReadMailbox: timeout")
		return msgList, fmt.Errorf("MailboxService->RequestReadMailbox: timeout")
	case <-d.TvBase.GetCtx().Done():
		log.Debugf("MailboxService->RequestReadMailbox: BaseService.GetCtx().Done()")
		return msgList, fmt.Errorf("MailboxService->RequestReadMailbox: BaseService.GetCtx().Done()")
	}
}

// DmsgServiceInterface
func (d *MailboxService) GetUserPubkeyHex() (string, error) {
	if d.lightMailboxUser == nil {
		return "", fmt.Errorf("MailboxService->GetUserPubkeyHex: user is nil")
	}
	return d.lightMailboxUser.Key.PubkeyHex, nil
}

func (d *MailboxService) GetUserSig(protoData []byte) ([]byte, error) {
	if d.lightMailboxUser == nil {
		log.Errorf("MailboxService->GetUserSig: user is nil")
		return nil, fmt.Errorf("MailboxService->GetUserSig: user is nil")
	}
	return d.lightMailboxUser.GetSig(protoData)
}

func (d *MailboxService) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	var target *dmsgUser.Target
	user := d.serviceUserList[pubkey]
	if user != nil {
		target = &user.Target
	} else if d.lightMailboxUser.Key.PubkeyHex == pubkey {
		target = &d.lightMailboxUser.Target
	}

	if target == nil {
		log.Errorf("MailboxService->GetPublishTarget: target is nil")
		return nil, fmt.Errorf("MailboxService->GetPublishTarget: target is nil")
	}
	return target, nil
}

// MailboxSpCallback
func (d *MailboxService) OnCreateMailboxRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("dmsgService->OnCreateMailboxRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		log.Errorf("dmsgService->OnCreateMailboxRequest: fail to convert requestProtoData to *pb.CreateMailboxReq")
		return nil, nil, false, fmt.Errorf("dmsgService->OnCreateMailboxRequest: fail to convert requestProtoData to *pb.CreateMailboxReq")
	}
	isAvailable := d.isAvailableMailbox(request.BasicData.Pubkey)
	if !isAvailable {
		log.Errorf("dmsgService->OnCreateMailboxRequest: exceeded the maximum number of mailbox service")
		return nil, nil, false, errors.New("dmsgService->OnCreateMailboxRequest: exceeded the maximum number of mailbox service")
	}
	user := d.getServiceUser(request.BasicData.Pubkey)
	if user != nil {
		log.Errorf("dmsgService->OnCreateMailboxRequest: pubkey is already exist in serviceUserList")
		retCode := &pb.RetCode{
			Code:   dmsgProtocol.AlreadyExistCode,
			Result: "dmsgService->OnCreateMailboxRequest: pubkey already exist in serviceUserList",
		}
		return nil, retCode, false, nil
	}

	err := d.subscribeServiceUser(request.BasicData.Pubkey)
	if err != nil {
		return nil, nil, false, err
	}
	log.Debugf("dmsgService->OnCreateMailboxRequest end")
	return nil, nil, false, nil
}

func (d *MailboxService) OnCreateMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debug("MailboxService->OnCreateMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	request, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		log.Errorf("MailboxService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.CreateMailboxReq")
		return nil, fmt.Errorf("MailboxService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.CreateMailboxReq")
	}
	response, ok := responseProtoData.(*pb.CreateMailboxRes)
	if !ok {
		log.Errorf("MailboxService->OnCreateMailboxResponse: fail to convert responseProtoData to *pb.CreateMailboxRes")
		return nil, fmt.Errorf("MailboxService->OnCreateMailboxResponse: fail to convert responseProtoData to *pb.CreateMailboxRes")
	}

	switch response.RetCode.Code {
	case 0: // new
		fallthrough
	case 1: // exist mailbox
		log.Debug("MailboxService->OnCreateMailboxResponse: mailbox has created, read message from mailbox...")
		err := d.releaseUnusedMailbox(response.BasicData.PeerID, request.BasicData.Pubkey)
		if err != nil {
			return nil, err
		}
	case -1:
		log.Warnf("MailboxService->OnCreateMailboxResponse: fail RetCode: %+v ", response.RetCode)
	default:
		log.Warnf("MailboxService->OnCreateMailboxResponse: other case RetCode: %+v", response.RetCode)
	}

	log.Debug("MailboxService->OnCreateMailboxResponse end")
	return nil, nil
}

func (d *MailboxService) OnReleaseMailboxRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("dmsgService->OnReleaseMailboxRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.ReleaseMailboxReq)
	if !ok {
		log.Errorf("dmsgService->OnReleaseMailboxRequest: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
		return nil, nil, false, fmt.Errorf("dmsgService->OnReleaseMailboxRequest: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
	}
	err := d.unsubscribeServiceUser(request.BasicData.Pubkey)
	if err != nil {
		return nil, nil, false, err
	}
	log.Debugf("dmsgService->OnReleaseMailboxRequest end")
	return nil, nil, false, nil
}

func (d *MailboxService) OnReleaseMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debug(
		"MailboxService->OnReleaseMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	_, ok := requestProtoData.(*pb.ReleaseMailboxReq)
	if !ok {
		log.Errorf("MailboxService->OnReleaseMailboxResponse: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
		return nil, fmt.Errorf("MailboxService->OnReleaseMailboxResponse: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
	}

	response, ok := responseProtoData.(*pb.ReleaseMailboxRes)
	if !ok {
		log.Errorf("MailboxService->OnReleaseMailboxResponse: fail to convert responseProtoData to *pb.ReleaseMailboxRes")
		return nil, fmt.Errorf("MailboxService->OnReleaseMailboxResponse: fail to convert responseProtoData to *pb.ReleaseMailboxRes")
	}
	if response.RetCode.Code < 0 {
		log.Warnf("MailboxService->OnReleaseMailboxResponse: fail RetCode: %+v", response.RetCode)
		return nil, fmt.Errorf("MailboxService->OnReleaseMailboxResponse: fail RetCode: %+v", response.RetCode)
	}

	log.Debug("MailboxService->OnReleaseMailboxResponse end")
	return nil, nil
}

func (d *MailboxService) OnReadMailboxMsgRequest(requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("dmsgService->OnReadMailboxMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.ReadMailboxReq)
	if !ok {
		log.Errorf("dmsgService->OnReadMailboxMsgRequest: fail to convert requestProtoData to *pb.ReadMailboxReq")
		return nil, nil, false, fmt.Errorf("dmsgService->OnReadMailboxMsgRequest: fail to convert requestProtoData to *pb.ReadMailboxReq")
	}

	pubkey := request.BasicData.Pubkey
	user := d.getServiceUser(pubkey)
	if user == nil {
		log.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot find user for pubkey: %s", pubkey)
		return nil, nil, false, fmt.Errorf("dmsgService->OnReadMailboxMsgRequest: cannot find user for pubkey: %s", pubkey)
	}
	user.MsgRWMutex.RLock()
	defer user.MsgRWMutex.RUnlock()

	var query = query.Query{
		Prefix: d.getMsgPrefix(pubkey),
	}
	results, err := d.datastore.Query(d.TvBase.GetCtx(), query)
	if err != nil {
		return nil, nil, false, err
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
			log.Errorf("dmsgService->OnReadMailboxMsgRequest: datastore.Delete error: %+v", err)
		}
	}

	if !find {
		log.Debug("dmsgService->OnReadMailboxMsgRequest: user msgs is empty")
	}
	log.Debugf("dmsgService->OnReadMailboxMsgRequest end")
	return mailboxMsgDataList, nil, false, nil
}

func (d *MailboxService) OnReadMailboxMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debug("MailboxService->OnReadMailboxMsgResponse: begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		log.Errorf("MailboxService->OnReadMailboxMsgResponse: fail to convert responseProtoData to *pb.ReadMailboxMsgRes")
		return nil, fmt.Errorf("MailboxService->OnReadMailboxMsgResponse: fail to convert responseProtoData to *pb.ReadMailboxMsgRes")
	}

	log.Debugf("MailboxService->OnReadMailboxMsgResponse: found (%d) new message", len(response.ContentList))

	msgList, err := d.parseReadMailboxResponse(responseProtoData, msg.MsgDirection.From)
	if err != nil {
		return nil, err
	}
	for _, msg := range msgList {
		log.Debugf("MailboxService->OnReadMailboxMsgResponse: From = %s, To = %s", msg.SrcPubkey, msg.DestPubkey)
		if d.onReceiveMsg != nil {
			d.onReceiveMsg(msg.SrcPubkey, msg.DestPubkey, msg.Content, msg.TimeStamp, msg.ID, msg.Direction)
		} else {
			log.Errorf("MailboxService->OnReadMailboxMsgResponse: OnReceiveMsg is nil")
		}
	}

	log.Debug("MailboxService->OnReadMailboxMsgResponse end")
	return nil, nil
}

// MailboxPpCallback
func (d *MailboxService) OnSeekMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debug("MailboxService->OnSeekMailboxRequest begin\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.SeekMailboxReq)
	if !ok {
		log.Errorf("MailboxService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.SeekMailboxReq")
		return nil, nil, false, fmt.Errorf("MailboxService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.SeekMailboxReq")
	}

	// not responding to self
	if request.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		log.Debugf("dmsgService->OnReleaseMailboxRequest: request.BasicData.PeerID == d.BaseService.GetHost().ID")
		return nil, nil, true, fmt.Errorf("dmsgService->OnReleaseMailboxRequest: request.BasicData.PeerID == d.BaseService.GetHost().ID")
	}

	log.Debug("MailboxService->OnSeekMailboxRequest end")
	return nil, nil, false, nil
}

func (d *MailboxService) OnSeekMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debug(
		"MailboxService->OnSeekMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	request, ok := requestProtoData.(*pb.SeekMailboxReq)
	if request == nil || !ok {
		log.Errorf("MailboxService->OnSeekMailboxResponse: fail to convert requestProtoData to *pb.SeekMailboxReq")
		return nil, fmt.Errorf("MailboxService->OnSeekMailboxResponse: fail to convert requestProtoData to *pb.SeekMailboxReq")
	}
	response, ok := responseProtoData.(*pb.SeekMailboxRes)
	if response == nil || !ok {
		log.Errorf("MailboxService->OnSeekMailboxResponse: fail to convert responseProtoData to *pb.SeekMailboxRes")
		return nil, fmt.Errorf("MailboxService->OnSeekMailboxResponse: fail to convert responseProtoData to *pb.SeekMailboxRes")
	}

	if request.BasicData.Pubkey != d.lightMailboxUser.Key.PubkeyHex {
		log.Errorf("MailboxService->OnSeekMailboxResponse: fail request.BasicData.Pubkey != d.lightMailboxUser.Key.PubkeyHex")
	}

	log.Debug("MailboxService->OnSeekMailboxResponse end")
	return nil, nil
}

func (d *MailboxService) OnPubsubMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("MailboxService->OnPubsubMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.SendMsgReq)
	if !ok {
		log.Errorf("MailboxService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
		return nil, nil, true, fmt.Errorf("MailboxService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.SendMsgReq")
	}

	if d.EnableService {
		pubkey := request.BasicData.Pubkey
		user := d.getServiceUser(pubkey)
		if user == nil {
			log.Errorf("MailboxService->OnPubsubMsgRequest: public key %s is not exist", pubkey)
			return nil, nil, true, fmt.Errorf("MailboxService->OnPubsubMsgRequest: public key %s is not exist", pubkey)
		}
		user.LastReciveTimestamp = time.Now().UnixNano()
		d.saveUserMsg(requestProtoData)
	}

	log.Debugf("MailboxService->OnPubsubMsgRequest end")
	return nil, nil, true, nil
}

func (d *MailboxService) OnPubsubMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf(
		"dmsgService->OnPubsubMsgResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	// never here
	log.Debugf("dmsgService->OnPubsubMsgResponse end")
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
							log.Errorf("dmsgService->readDestUserPubsub: query error: %v", err)
						}

						for result := range results.Next() {
							d.datastore.Delete(d.TvBase.GetCtx(), datastore.NewKey(result.Key))
							log.Debugf("dmsgService->readDestUserPubsub: delete msg by key:%v", string(result.Key))
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
		protocolID, protocolData, protocolHandle, err := d.WaitPubsubProtocolData(target)
		if err != nil {
			log.Warnf("MailboxService->handlePubsubProtocol: target.WaitMsg error: %+v", err)
			return
		}

		if protocolHandle == nil {
			continue
		}

		msgRequestPID := d.pubsubMsgProtocol.Adapter.GetRequestPID()
		msgResponsePID := d.pubsubMsgProtocol.Adapter.GetResponsePID()
		seekRequestPID := d.seekMailboxProtocol.Adapter.GetRequestPID()
		seekResponsePID := d.seekMailboxProtocol.Adapter.GetResponsePID()
		log.Debugf("MailboxService->handlePubsubProtocol: protocolID: %d", protocolID)

		switch protocolID {
		case msgRequestPID:
			err = protocolHandle.HandleRequestData(protocolData)
			if err != nil {
				log.Warnf("MailboxService->handlePubsubProtocol: HandleRequestData error: %v", err)
			}
			continue
		case msgResponsePID:
			continue
		case seekRequestPID:
			err = protocolHandle.HandleRequestData(protocolData)
			if err != nil {
				log.Warnf("MailboxService->handlePubsubProtocol: HandleRequestData error: %v", err)
			}
			continue
		case seekResponsePID:
			err = protocolHandle.HandleResponseData(protocolData)
			if err != nil {
				log.Warnf("MailboxService->handlePubsubProtocol: HandleResponseData error: %v", err)
			}
			continue
		}
	}
}

// user
func (d *MailboxService) initUser(pubkeyData []byte, getSig dmsgKey.GetSigCallback) error {
	log.Debug("MailboxService->initUser begin")
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

	log.Debug("MailboxService->initUser end")
	return nil
}

func (d *MailboxService) subscribeUser(pubkey string, getSig dmsgKey.GetSigCallback) error {
	log.Debugf("MailboxService->subscribeUser begin\npubkey: %s", pubkey)
	if d.lightMailboxUser != nil {
		log.Errorf("MailboxService->subscribeUser: user isn't nil")
		return fmt.Errorf("MailboxService->subscribeUser: user isn't nil")
	}

	if d.serviceUserList[pubkey] != nil {
		log.Errorf("MailboxService->subscribeUser: pubkey is already exist in serviceUserList")
		return fmt.Errorf("MailboxService->subscribeUser: pubkey is already exist in serviceUserList")
	}

	target, err := dmsgUser.NewTarget(d.TvBase.GetCtx(), pubkey, getSig)
	if err != nil {
		log.Errorf("MailboxService->subscribeUser: NewUser error: %v", err)
		return err
	}

	err = target.InitPubsub(pubkey)
	if err != nil {
		log.Errorf("MailboxService->subscribeUser: InitPubsub error: %v", err)
		return err
	}

	d.lightMailboxUser = &dmsgUser.LightMailboxUser{
		Target:        *target,
		ServicePeerID: "",
	}

	log.Debugf("MailboxService->subscribeUser end")
	return nil
}

func (d *MailboxService) unsubscribeUser() error {
	log.Debugf("MailboxService->unsubscribeUser begin")
	if d.lightMailboxUser == nil {
		log.Errorf("MailboxService->unsubscribeUser: userPubkey is not exist in destUserInfoList")
		return fmt.Errorf("MailboxService->unsubscribeUser: userPubkey is not exist in destUserInfoList")
	}
	d.lightMailboxUser.Close()
	d.lightMailboxUser = nil
	log.Debugf("MailboxService->unsubscribeUser end")
	return nil
}

// dest user
func (d *MailboxService) getServiceUser(pubkey string) *dmsgUser.ServiceMailboxUser {
	return d.serviceUserList[pubkey]
}

func (d *MailboxService) subscribeServiceUser(pubkey string) error {
	log.Debug("MailboxService->subscribeServiceUser begin\npubkey: %s", pubkey)
	if d.serviceUserList[pubkey] != nil {
		log.Errorf("MailboxService->subscribeServiceUser: pubkey is already exist in serviceUserList")
		return fmt.Errorf("MailboxService->subscribeServiceUser: pubkey is already exist in serviceUserList")
	}
	target, err := dmsgUser.NewTarget(d.TvBase.GetCtx(), pubkey, nil)
	if err != nil {
		log.Errorf("MailboxService->subscribeServiceUser: NewTarget error: %v", err)
		return err
	}
	err = target.InitPubsub(pubkey)
	if err != nil {
		log.Errorf("MailboxService->subscribeServiceUser: InitPubsub error: %v", err)
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
	go d.handlePubsubProtocol(&user.Target)
	return nil
}

func (d *MailboxService) unsubscribeServiceUser(pubkey string) error {
	log.Debugf("MailboxService->unsubscribeServiceUser begin\npubkey: %s", pubkey)

	user := d.serviceUserList[pubkey]
	if user == nil {
		log.Errorf("MailboxService->unsubscribeServiceUser: pubkey is not exist in serviceUserList")
		return fmt.Errorf("MailboxService->unsubscribeServiceUser: pubkey is not exist in serviceUserList")
	}
	user.Close()
	delete(d.serviceUserList, pubkey)

	log.Debug("MailboxService->unsubscribeServiceUser end")
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
	log.Debug("MailboxService->initMailbox begin")
	_, seekMailboxDoneChan, err := d.seekMailboxProtocol.Request(d.lightMailboxUser.Key.PubkeyHex, d.lightMailboxUser.Key.PubkeyHex)
	if err != nil {
		return
	}
	select {
	case seekMailboxResponseProtoData := <-seekMailboxDoneChan:
		log.Debugf("MailboxService->initMailbox: seekMailboxProtoData: %+v", seekMailboxResponseProtoData)
		response, ok := seekMailboxResponseProtoData.(*pb.SeekMailboxRes)
		if !ok || response == nil {
			log.Errorf("MailboxService->initMailbox: seekMailboxProtoData is not SeekMailboxRes")
			// skip seek when seek mailbox quest fail (server err), create a new mailbox
		}
		if response.RetCode.Code < 0 {
			log.Errorf("MailboxService->initMailbox: seekMailboxProtoData fail")
			// skip seek when seek mailbox quest fail, create a new mailbox
		} else {
			log.Debugf("MailboxService->initMailbox: seekMailboxProtoData success")
			go d.releaseUnusedMailbox(response.BasicData.PeerID, pubkey)
			return
		}
	case <-time.After(3 * time.Second):
		log.Debugf("MailboxService->initMailbox: time.After 3s, create new mailbox")
		// begin create new mailbox

		hostId := d.TvBase.GetHost().ID().String()
		servicePeerList, err := d.TvBase.GetAvailableServicePeerList(hostId)
		if err != nil {
			log.Errorf("MailboxService->initMailbox: getAvailableServicePeerList error: %v", err)
			return
		}

		peerID := d.TvBase.GetHost().ID().String()
		for _, servicePeerID := range servicePeerList {
			log.Debugf("MailboxService->initMailbox: servicePeerID: %v", servicePeerID)
			if peerID == servicePeerID.String() {
				continue
			}
			_, createMailboxDoneChan, err := d.createMailboxProtocol.Request(servicePeerID, pubkey)
			if err != nil {
				log.Errorf("MailboxService->initMailbox: createMailboxProtocol.Request error: %v", err)
				continue
			}

			select {
			case createMailboxResponseProtoData := <-createMailboxDoneChan:
				log.Debugf("MailboxService->initMailbox: createMailboxResponseProtoData: %+v", createMailboxResponseProtoData)
				response, ok := createMailboxResponseProtoData.(*pb.CreateMailboxRes)
				if !ok || response == nil {
					log.Errorf("MailboxService->initMailbox: createMailboxDoneChan is not CreateMailboxRes")
					continue
				}

				switch response.RetCode.Code {
				case 0, 1:
					log.Debugf("MailboxService->initMailbox: createMailboxProtocol success")
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

		log.Error("MailboxService->InitUser: no available service peers")
		return
		// end create mailbox
	case <-d.TvBase.GetCtx().Done():
		log.Debug("MailboxService->InitUser: BaseService.GetCtx().Done()")
		return
	}
}

func (d *MailboxService) releaseUnusedMailbox(peerIdHex string, pubkey string) error {
	log.Debug("MailboxService->releaseUnusedMailbox begin")

	peerID, err := peer.Decode(peerIdHex)
	if err != nil {
		log.Warnf("MailboxService->releaseUnusedMailbox: fail to decode peer id: %v", err)
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
				log.Debugf("MailboxService->releaseUnusedMailbox: releaseMailboxDoneChan success")
			case <-time.After(time.Second * 3):
				return fmt.Errorf("MailboxService->releaseUnusedMailbox: releaseMailboxDoneChan time out")
			case <-d.TvBase.GetCtx().Done():
				return fmt.Errorf("MailboxService->releaseUnusedMailbox: BaseService.GetCtx().Done()")
			}
		}
	case <-time.After(time.Second * 10):
		return fmt.Errorf("MailboxService->releaseUnusedMailbox: readMailboxDoneChan time out")
	case <-d.TvBase.GetCtx().Done():
		return fmt.Errorf("MailboxService->releaseUnusedMailbox: BaseService.GetCtx().Done()")
	}
	log.Debugf("MailboxService->releaseUnusedMailbox end")
	return nil
}

func (d *MailboxService) parseReadMailboxResponse(responseProtoData protoreflect.ProtoMessage, direction string) ([]msg.Msg, error) {
	log.Debugf("MailboxService->parseReadMailboxResponse begin:\nresponseProtoData: %v", responseProtoData)
	msgList := []msg.Msg{}
	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		log.Errorf("MailboxService->parseReadMailboxResponse: fail to convert to *pb.ReadMailboxMsgRes")
		return msgList, fmt.Errorf("MailboxService->parseReadMailboxResponse: fail to convert to *pb.ReadMailboxMsgRes")
	}

	for _, mailboxItem := range response.ContentList {
		log.Debugf("MailboxService->parseReadMailboxResponse: msg key = %s", mailboxItem.Key)
		msgContent := mailboxItem.Content

		fields := strings.Split(mailboxItem.Key, msg.MsgKeyDelimiter)
		if len(fields) < msg.MsgFieldsLen {
			log.Errorf("MailboxService->parseReadMailboxResponse: msg key fields len not enough")
			return msgList, fmt.Errorf("MailboxService->parseReadMailboxResponse: msg key fields len not enough")
		}

		timeStamp, err := strconv.ParseInt(fields[msg.MsgTimeStampIndex], 10, 64)
		if err != nil {
			log.Errorf("MailboxService->parseReadMailboxResponse: msg timeStamp parse error : %v", err)
			return msgList, fmt.Errorf("MailboxService->parseReadMailboxResponse: msg timeStamp parse error : %v", err)
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
	log.Debug("MailboxService->parseReadMailboxResponse end")
	return msgList, nil
}

func (d *MailboxService) saveUserMsg(protoMsg protoreflect.ProtoMessage) error {
	sendMsgReq, ok := protoMsg.(*pb.SendMsgReq)
	if !ok {
		log.Errorf("dmsgService->saveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
		return fmt.Errorf("dmsgService->saveUserMsg: cannot convert %v to *pb.SendMsgReq", protoMsg)
	}

	pubkey := sendMsgReq.BasicData.Pubkey
	user := d.getServiceUser(pubkey)
	if user == nil {
		log.Errorf("dmsgService->saveUserMsg: cannot find src user pubsub for %v", pubkey)
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
