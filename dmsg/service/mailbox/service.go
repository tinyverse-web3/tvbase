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
	"github.com/tinyverse-web3/tvbase/common/db"
	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	dmsgCommonUtil "github.com/tinyverse-web3/tvbase/dmsg/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
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
	stopReadMailbox       chan bool
}

func CreateService(tvbaseService define.TvBaseService) (*MailboxService, error) {
	d := &MailboxService{}
	err := d.Init(tvbaseService)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *MailboxService) Init(tvbaseService define.TvBaseService) error {
	err := d.BaseService.Init(tvbaseService)
	if err != nil {
		return err
	}
	d.serviceUserList = make(map[string]*dmsgUser.ServiceMailboxUser)
	return nil
}

// sdk-common
func (d *MailboxService) IsExistMailbox(userPubkey string, timeout time.Duration) (*pb.SeekMailboxRes, error) {
	_, seekMailboxDoneChan, err := d.seekMailboxProtocol.Request(d.lightMailboxUser.Key.PubkeyHex, userPubkey)
	if err != nil {
		return nil, fmt.Errorf("MailboxService->IsExistMailbox: seekMailboxProtocol.Request error : %+v", err)
	}

	select {
	case seekMailboxResponseProtoData := <-seekMailboxDoneChan:
		response, ok := seekMailboxResponseProtoData.(*pb.SeekMailboxRes)
		if !ok || response == nil {
			return response, fmt.Errorf("MailboxService->IsExistMailbox: seekMailboxProtoData is not SeekMailboxRes")
		}
		if response.RetCode.Code < 0 {
			return response, fmt.Errorf("MailboxService->IsExistMailbox: seekMailboxProtoData retcode.code < 0")
		} else {
			log.Debugf("MailboxService->IsExistMailbox: seekMailboxProtoData success")
			return response, nil
		}
	case <-time.After(timeout):
		return nil, fmt.Errorf("MailboxService->IsExistMailbox: time.After 3s timeout")
	case <-d.BaseService.TvBase.GetCtx().Done():
		log.Debug("MailboxService->IsExistMailbox: BaseService.GetCtx().Done()")
	}
	return nil, fmt.Errorf("MailboxService->IsExistMailbox: unknow error")
}

func (d *MailboxService) Start(enableRequest bool, pubkey string, getSig dmsgKey.GetSigCallback) error {
	log.Debugf("MailboxService->Start begin")

	err := d.SubscribeUser(pubkey, getSig)
	if err != nil {
		return err
	}

	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()
	// stream protocol
	d.createMailboxProtocol = adapter.NewCreateMailboxProtocol(ctx, host, d, d, enableRequest, pubkey)
	d.releaseMailboxPrtocol = adapter.NewReleaseMailboxProtocol(ctx, host, d, d, enableRequest, pubkey)
	d.readMailboxMsgPrtocol = adapter.NewReadMailboxMsgProtocol(ctx, host, d, d, enableRequest, pubkey)

	cfg := d.BaseService.TvBase.GetConfig()
	filepath := d.BaseService.TvBase.GetRootPath() + cfg.DMsg.DatastorePath
	d.datastore, err = db.CreateBadgerDB(filepath)
	if err != nil {
		log.Errorf("MailboxService->Start: create datastore error %v", err)
		return err
	}

	// pubsub protocol
	d.seekMailboxProtocol = adapter.NewSeekMailboxProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(d.seekMailboxProtocol.Adapter.GetResponsePID(), d.seekMailboxProtocol)
	d.pubsubMsgProtocol = adapter.NewPubsubMsgProtocol(ctx, host, d, d)
	d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetResponsePID(), d.pubsubMsgProtocol)

	d.RegistPubsubProtocol(d.seekMailboxProtocol.Adapter.GetRequestPID(), d.seekMailboxProtocol)
	d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetRequestPID(), d.pubsubMsgProtocol)

	d.stopCleanRestResource = make(chan bool)
	d.cleanRestServiceUser(12 * time.Hour)

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
	select {
	case d.stopCleanRestResource <- true:
		log.Debugf("MailboxService->Stop: succ send stopCleanRestResource")
	default:
		log.Debugf("MailboxService->Stop: no receiver for stopCleanRestResource")
	}
	close(d.stopCleanRestResource)
	close(d.stopReadMailbox)
	log.Debug("MailboxService->Stop end")
	return nil
}

func (d *MailboxService) SetOnReceiveMsg(cb msg.OnReceiveMsg) {
	d.onReceiveMsg = cb
}

// sdk-msg
func (d *MailboxService) ReadMailbox(timeout time.Duration) ([]msg.ReceiveMsg, error) {
	if d.lightMailboxUser == nil {
		log.Errorf("MailboxService->ReadMailbox: user is nil")
		return nil, fmt.Errorf("MailboxService->ReadMailbox: user is nil")
	}
	if d.lightMailboxUser.ServicePeerID == "" {
		log.Errorf("MailboxService->ReadMailbox: servicePeerID is empty")
		return nil, fmt.Errorf("MailboxService->ReadMailbox: servicePeerID is empty")
	}
	return d.readMailbox(
		d.lightMailboxUser.ServicePeerID,
		d.lightMailboxUser.Key.PubkeyHex,
		timeout,
		false,
	)
}

// DmsgService
func (d *MailboxService) GetUserPubkeyHex() (string, error) {
	if d.lightMailboxUser == nil {
		log.Errorf("MailboxService->GetUserPubkeyHex: user is nil")
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

	topicName := target.Pubsub.Topic.String()
	log.Debugf("MailboxService->GetPublishTarget: target's topic name: %s", topicName)
	return target, nil
}

// MailboxSpCallback
func (d *MailboxService) OnCreateMailboxRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("MailboxService->OnCreateMailboxRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		log.Errorf("MailboxService->OnCreateMailboxRequest: fail to convert requestProtoData to *pb.CreateMailboxReq")
		return nil, nil, false, fmt.Errorf("MailboxService->OnCreateMailboxRequest: fail to convert requestProtoData to *pb.CreateMailboxReq")
	}

	if request.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		log.Debugf("MailboxService->OnCreatePubusubRequest: request.BasicData.PeerID == d.TvBase.GetHost().ID().String()")
		return nil, nil, true, nil
	}

	isAvailable := d.isAvailableMailbox(request.BasicData.Pubkey)
	if !isAvailable {
		log.Errorf("MailboxService->OnCreateMailboxRequest: exceeded the maximum number of mailbox service")
		return nil, nil, false, errors.New("MailboxService->OnCreateMailboxRequest: exceeded the maximum number of mailbox service")
	}
	pubkey := request.BasicData.Pubkey
	if request.BasicData.ProxyPubkey != "" {
		pubkey = request.BasicData.ProxyPubkey
	}

	user := d.getServiceUser(pubkey)
	if user != nil {
		log.Errorf("MailboxService->OnCreateMailboxRequest: pubkey is already exist in serviceUserList")
		retCode := &pb.RetCode{
			Code:   dmsgProtocol.AlreadyExistCode,
			Result: "MailboxService->OnCreateMailboxRequest: pubkey already exist in serviceUserList",
		}
		return nil, retCode, false, nil
	}

	err := d.subscribeServiceUser(request.BasicData.Pubkey)
	if err != nil {
		return nil, nil, false, err
	}
	log.Debugf("MailboxService->OnCreateMailboxRequest end")
	return nil, nil, false, nil
}

func (d *MailboxService) OnCreateMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf("MailboxService->OnCreateMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	request, ok := requestProtoData.(*pb.CreateMailboxReq)
	if !ok {
		log.Debugf("MailboxService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.CreateMailboxReq")
		// return nil, fmt.Errorf("MailboxService->OnCreateMailboxResponse: fail to convert requestProtoData to *pb.CreateMailboxReq")
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
		err := d.releaseUnusedMailbox(response.BasicData.PeerID, request.BasicData.Pubkey, 30*time.Second)
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
	log.Debugf("MailboxService->OnReleaseMailboxRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.ReleaseMailboxReq)
	if !ok {
		log.Errorf("MailboxService->OnReleaseMailboxRequest: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
		return nil, nil, false, fmt.Errorf("MailboxService->OnReleaseMailboxRequest: fail to convert requestProtoData to *pb.ReleaseMailboxReq")
	}

	if request.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		log.Debugf("MailboxService->OnCreatePubusubRequest: request.BasicData.PeerID == d.TvBase.GetHost().ID().String()")
		return nil, nil, true, nil
	}

	pubkey := request.BasicData.Pubkey
	if request.BasicData.ProxyPubkey != "" {
		pubkey = request.BasicData.ProxyPubkey
	}

	err := d.unsubscribeServiceUser(pubkey)
	if err != nil {
		return nil, nil, false, err
	}
	log.Debugf("MailboxService->OnReleaseMailboxRequest end")
	return nil, nil, false, nil
}

func (d *MailboxService) OnReleaseMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debug(
		"MailboxService->OnReleaseMailboxResponse begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
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

func (d *MailboxService) OnReadMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("MailboxService->OnReadMailboxRequest begin:\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.ReadMailboxReq)
	if !ok {
		log.Errorf("MailboxService->OnReadMailboxRequest: fail to convert requestProtoData to *pb.ReadMailboxReq")
		return nil, nil, false, fmt.Errorf("MailboxService->OnReadMailboxRequest: fail to convert requestProtoData to *pb.ReadMailboxReq")
	}

	if request.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		log.Debugf("MailboxService->OnCreatePubusubRequest: request.BasicData.PeerID == d.TvBase.GetHost().ID().String()")
		return nil, nil, true, nil
	}

	pubkey := request.BasicData.Pubkey
	if request.BasicData.ProxyPubkey != "" {
		pubkey = request.BasicData.ProxyPubkey
	}
	user := d.getServiceUser(pubkey)
	if user == nil {
		log.Errorf("MailboxService->OnReadMailboxRequest: cannot find user for pubkey: %s", pubkey)
		return nil, nil, false, fmt.Errorf("MailboxService->OnReadMailboxRequest: cannot find user for pubkey: %s", pubkey)
	}

	var query = query.Query{
		Prefix: d.getMsgPrefix(pubkey),
	}
	user.MsgRWMutex.Lock()
	defer user.MsgRWMutex.Unlock()
	results, err := d.datastore.Query(d.TvBase.GetCtx(), query)
	if err != nil {
		return nil, nil, false, err
	}
	defer results.Close()

	mailboxMsgDataList := []*pb.MailboxItem{}
	find := false
	needDeleteKeyList := []string{}
	const MaxContentSize = 2 * 1024 * 1024
	factSize := 0
	requestParam := &adapter.ReadMailRequestParam{}
	for result := range results.Next() {
		if !request.ClearMode {
			mailboxMsgData := &pb.MailboxItem{
				Key:     string(result.Key),
				Content: result.Value,
			}
			mailboxMsgDataList = append(mailboxMsgDataList, mailboxMsgData)
			factSize += len(result.Value)
			if factSize >= MaxContentSize {
				for range results.Next() {
					requestParam.ExistData = true
					break
				}
				break
			}
		}
		needDeleteKeyList = append(needDeleteKeyList, string(result.Key))
		find = true
	}

	requestParam.ItemList = mailboxMsgDataList
	for _, needDeleteKey := range needDeleteKeyList {
		err := d.datastore.Delete(d.TvBase.GetCtx(), datastore.NewKey(needDeleteKey))
		if err != nil {
			log.Errorf("MailboxService->OnReadMailboxRequest: datastore.Delete error: %+v", err)
		}
	}

	if !find {
		log.Debug("MailboxService->OnReadMailboxRequest: user msgs is empty")
	}
	user.LastTimestamp = time.Now().UnixNano()
	log.Debugf("MailboxService->OnReadMailboxRequest end")
	return requestParam, nil, false, nil
}

func (d *MailboxService) OnReadMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf("MailboxService->OnReadMailboxResponse: begin\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)

	response, ok := responseProtoData.(*pb.ReadMailboxRes)
	if response == nil || !ok {
		log.Errorf("MailboxService->OnReadMailboxResponse: fail to convert responseProtoData to *pb.ReadMailboxMsgRes")
		return nil, fmt.Errorf("MailboxService->OnReadMailboxResponse: fail to convert responseProtoData to *pb.ReadMailboxMsgRes")
	}

	log.Debugf("MailboxService->OnReadMailboxResponse: found (%d) new message", len(response.ContentList))

	msgList, err := d.parseReadMailboxResponse(responseProtoData, msg.MsgDirection.From)
	if err != nil {
		return nil, err
	}
	for _, msg := range msgList {
		log.Debugf("MailboxService->OnReadMailboxResponse: From = %s, To = %s", msg.ReqPubkey, msg.DestPubkey)
		if d.onReceiveMsg != nil {
			d.onReceiveMsg(&msg)
		} else {
			log.Warnf("MailboxService->OnReadMailboxResponse: callback func onReadMailmsg is nil")
		}
	}

	log.Debug("MailboxService->OnReadMailboxResponse end")
	return nil, nil
}

// MailboxPpCallback
func (d *MailboxService) OnSeekMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("MailboxService->OnSeekMailboxRequest begin\nrequestProtoData: %+v", requestProtoData)
	request, ok := requestProtoData.(*pb.SeekMailboxReq)
	if !ok {
		log.Errorf("MailboxService->OnSeekMailboxRequest: fail to convert requestProtoData to *pb.SeekMailboxReq")
		return nil, nil, false, fmt.Errorf("MailboxService->OnSeekMailboxRequest: fail to convert requestProtoData to *pb.SeekMailboxReq")
	}

	// no responding to self
	if request.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		log.Debugf("MailboxService->OnSeekMailboxRequest: request.BasicData.PeerID == d.TvBase.GetHost().ID()")
		return nil, nil, true, nil
	}

	pubkey := request.BasicData.Pubkey
	user := d.getServiceUser(pubkey)
	if user == nil {
		log.Errorf("MailboxService->OnSeekMailboxRequest: cannot find user for pubkey: %s", pubkey)
		retCode := &pb.RetCode{
			Code:   dmsgProtocol.NoExistCode,
			Result: "MailboxService->OnSeekMailboxRequest: pubkey no exist in serviceUserList",
		}
		return nil, retCode, false, fmt.Errorf("MailboxService->OnSeekMailboxRequest: cannot find user for pubkey: %s", pubkey)
	}

	log.Debug("MailboxService->OnSeekMailboxRequest end")
	return nil, nil, false, nil
}

func (d *MailboxService) OnSeekMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf(
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
	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Errorf("MailboxService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
		return nil, nil, true, fmt.Errorf("MailboxService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
	}

	if request.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
		log.Debugf("MailboxService->OnCreatePubusubRequest: request.BasicData.PeerID == d.TvBase.GetHost().ID().String()")
		return nil, nil, true, nil
	}

	pubkey := request.BasicData.Pubkey
	user := d.getServiceUser(pubkey)
	if user == nil {
		log.Errorf("MailboxService->OnPubsubMsgRequest: public key %s is not exist", pubkey)
		return nil, nil, true, fmt.Errorf("MailboxService->OnPubsubMsgRequest: public key %s is not exist", pubkey)
	}
	user.LastTimestamp = time.Now().UnixNano()
	d.saveMsg(requestProtoData)

	log.Debugf("MailboxService->OnPubsubMsgRequest end")
	return nil, nil, true, nil
}

func (d *MailboxService) OnPubsubMsgResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	log.Debugf(
		"MailboxService->OnPubsubMsgResponse begin:\nrequestProtoData: %+v\nresponseProtoData: %+v",
		requestProtoData, responseProtoData)
	// never here
	log.Debugf("MailboxService->OnPubsubMsgResponse end")
	return nil, nil
}

// common
func (d *MailboxService) cleanRestServiceUser(dur time.Duration) {
	go func() {
		serviceTicker := time.NewTicker(dur)
		defer serviceTicker.Stop()
		for {
			select {
			case <-d.stopCleanRestResource:
				return
			case <-serviceTicker.C:
				for pubkey, pubsub := range d.serviceUserList {
					days := dmsgCommonUtil.DaysBetween(pubsub.LastTimestamp, time.Now().UnixNano())
					// delete mailbox msg in datastore and unsubscribe mailbox when days is over
					if days >= d.GetConfig().KeepMailboxDay {
						var query = query.Query{
							Prefix:   d.getMsgPrefix(pubkey),
							KeysOnly: true,
						}
						user := d.getServiceUser(pubkey)
						if user == nil {
							log.Errorf("MailboxService->cleanRestServiceUser: cannot find user for pubkey: %s", pubkey)
							continue
						}
						func() {
							user.MsgRWMutex.Lock()
							defer user.MsgRWMutex.Unlock()
							results, err := d.datastore.Query(d.TvBase.GetCtx(), query)
							if err != nil {

								log.Errorf("MailboxService->cleanRestServiceUser: query error: %v", err)
							}

							for result := range results.Next() {
								err = d.datastore.Delete(d.TvBase.GetCtx(), datastore.NewKey(result.Key))
								if err != nil {
									log.Errorf("MailboxService->cleanRestServiceUser: datastore.Delete error: %+v", err)
								}
								log.Debugf("MailboxService->cleanRestServiceUser: delete msg by key:%v", string(result.Key))
							}
						}()
						d.unsubscribeServiceUser(pubkey)
					}
				}
				continue
			case <-d.TvBase.GetCtx().Done():
				return
			}
		}
	}()
}

func (d *MailboxService) TickReadMailbox(checkDuration time.Duration, readMailboxTimeout time.Duration) {
	if d.stopReadMailbox != nil {
		d.stopReadMailbox <- true
		close(d.stopReadMailbox)
	} else {
		d.stopReadMailbox = make(chan bool)
	}

	go func() {
		ticker := time.NewTicker(checkDuration)
		defer ticker.Stop()
		for {
			select {
			case <-d.stopReadMailbox:
				return
			case <-ticker.C:
				_, err := d.readMailbox(d.lightMailboxUser.ServicePeerID, d.lightMailboxUser.Key.PubkeyHex, readMailboxTimeout, true)
				if err != nil {
					log.Errorf("MailboxService->tickReadMailbox: readMailbox error: %v", err)
					continue
				}
			case <-d.TvBase.GetCtx().Done():
				return
			}
		}
	}()
}

func (d *MailboxService) handlePubsubProtocol(target *dmsgUser.Target) error {
	ctx := d.TvBase.GetCtx()
	protocolDataChan, err := dmsgServiceCommon.WaitMessage(ctx, target.Key.PubkeyHex)
	if err != nil {
		return err
	}
	log.Debugf("MailboxService->handlePubsubProtocol: protocolDataChan: %+v", protocolDataChan)
	go func() {
		for {
			select {
			case protocolHandle, ok := <-protocolDataChan:
				if !ok {
					return
				}
				pid := protocolHandle.PID
				log.Debugf("MailboxService->handlePubsubProtocol: \npid: %d\ntopicName: %s", pid, target.Pubsub.Topic.String())

				handle := d.ProtocolHandleList[pid]
				if handle == nil {
					log.Debugf("MailboxService->handlePubsubProtocol: no handle for pid: %d", pid)
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
						log.Warnf("MailboxService->handlePubsubProtocol: HandleRequestData error: %v", err)
					}
					continue
				case msgResponsePID:
					continue
				case seekRequestPID:
					err = handle.HandleRequestData(data)
					if err != nil {
						log.Warnf("MailboxService->handlePubsubProtocol: HandleRequestData error: %v", err)
					}
					continue
				case seekResponsePID:
					err = handle.HandleResponseData(data)
					if err != nil {
						log.Warnf("MailboxService->handlePubsubProtocol: HandleResponseData error: %v", err)
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

// user
func (d *MailboxService) SubscribeUser(pubkey string, getSig dmsgKey.GetSigCallback) error {
	log.Debugf("MailboxService->SubscribeUser begin\npubkey: %s", pubkey)
	if d.lightMailboxUser != nil {
		log.Errorf("MailboxService->SubscribeUser: user isn't nil")
		return fmt.Errorf("MailboxService->SubscribeUser: user isn't nil")
	}

	if d.serviceUserList[pubkey] != nil {
		log.Errorf("MailboxService->SubscribeUser: pubkey is already exist in serviceUserList")
		return fmt.Errorf("MailboxService->SubscribeUser: pubkey is already exist in serviceUserList")
	}

	target, err := dmsgUser.NewTarget(pubkey, getSig)
	if err != nil {
		log.Errorf("MailboxService->SubscribeUser: NewUser error: %v", err)
		return err
	}

	err = target.InitPubsub(pubkey)
	if err != nil {
		log.Errorf("MailboxService->SubscribeUser: InitPubsub error: %v", err)
		return err
	}

	user := &dmsgUser.LightMailboxUser{
		Target:        *target,
		ServicePeerID: "",
	}

	err = d.handlePubsubProtocol(&user.Target)
	if err != nil {
		log.Errorf("MailboxService->SubscribeUser: handlePubsubProtocol error: %v", err)
		err := user.Target.Close()
		if err != nil {
			log.Warnf("MailboxService->SubscribeUser: Target.Close error: %v", err)
			return err
		}
		return err
	}
	d.lightMailboxUser = user
	log.Debugf("MailboxService->SubscribeUser end")
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
	target, err := dmsgUser.NewTarget(pubkey, nil)
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
			Target:        *target,
			LastTimestamp: time.Now().UnixNano(),
		},
		MsgRWMutex: sync.RWMutex{},
	}

	err = d.handlePubsubProtocol(&user.Target)
	if err != nil {
		log.Errorf("MailboxService->subscribeServiceUser: handlePubsubProtocol error: %v", err)
		err := user.Target.Close()
		if err != nil {
			log.Warnf("MailboxService->subscribeServiceUser: Target.Close error: %v", err)
			return err
		}
		return err
	}
	d.serviceUserList[pubkey] = user
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
	return destUserCount < d.GetConfig().MaxMailboxCount
}

func (d *MailboxService) createMailbox(pubkey string, timeout time.Duration) error {
	hostId := d.TvBase.GetHost().ID().String()
	servicePeerList, err := d.TvBase.GetAvailableServicePeerList(hostId)
	if err != nil {
		log.Errorf("MailboxService->createMailbox: getAvailableServicePeerList error: %v", err)
		return err
	}

	peerID := d.TvBase.GetHost().ID().String()
	for _, servicePeerID := range servicePeerList {
		log.Debugf("MailboxService->createMailbox: servicePeerID: %v", servicePeerID)
		if peerID == servicePeerID.String() {
			continue
		}
		_, createMailboxResponseChan, err := d.createMailboxProtocol.Request(servicePeerID, pubkey)
		if err != nil {
			log.Errorf("MailboxService->createMailbox: createMailboxProtocol.Request error: %v", err)
			continue
		}

		select {
		case createMailboxResponseProtoData := <-createMailboxResponseChan:
			log.Debugf("MailboxService->createMailbox: createMailboxResponseProtoData: %+v", createMailboxResponseProtoData)
			response, ok := createMailboxResponseProtoData.(*pb.CreateMailboxRes)
			if !ok || response == nil {
				log.Errorf("MailboxService->createMailbox: createMailboxResponseChan is not CreateMailboxRes")
				continue
			}

			switch response.RetCode.Code {
			case 0, 1:
				log.Debugf("MailboxService->createMailbox: createMailboxProtocol success")
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
	log.Error("MailboxService->createMailbox: no available service peers")
	return nil
}

func (d *MailboxService) CreateMailbox(pubkey string, timeout time.Duration) error {
	log.Debug("MailboxService->CreateMailbox begin")
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
		return fmt.Errorf("MailboxService->CreateMailboxWithProxy: timeout")
	}
	return nil
}

func (d *MailboxService) CreateUserMailbox(timeout time.Duration) error {
	return d.CreateMailbox(d.lightMailboxUser.Key.PubkeyHex, timeout)
}

func (d *MailboxService) CreateProxyMailbox(pubkey string, timeout time.Duration) error {
	return d.CreateMailbox(pubkey, timeout)
}

func (d *MailboxService) readMailbox(peerIdHex string, reqPubkey string, timeout time.Duration, clearMode bool) ([]msg.ReceiveMsg, error) {
	var msgList []msg.ReceiveMsg
	peerID, err := peer.Decode(peerIdHex)
	if err != nil {
		log.Errorf("MailboxService->readMailbox: fail to decode peer id: %v", err)
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
				return msgList, fmt.Errorf("MailboxService->readMailbox: response:%+v", response)
			}
			if response.RetCode.Code < 0 {
				return msgList, fmt.Errorf("MailboxService->readMailbox: readMailboxRes fail")
			}
			receiveMsglist, err := d.parseReadMailboxResponse(response, msg.MsgDirection.From)
			if err != nil {
				return msgList, err
			}
			msgList = append(msgList, receiveMsglist...)
			if !response.ExistData {
				log.Debugf("MailboxService->readMailbox: readMailboxChanResponseChan success")
				return msgList, nil
			}
			continue
		case <-time.After(timeout):
			return msgList, fmt.Errorf("MailboxService->readMailbox: readMailboxResponseChan time out")
		case <-d.TvBase.GetCtx().Done():
			return msgList, fmt.Errorf("MailboxService->readMailbox: BaseService.GetCtx().Done()")
		}
	}
}

func (d *MailboxService) releaseUnusedMailbox(peerIdHex string, reqPubkey string, timeout time.Duration) error {
	log.Debug("MailboxService->releaseUnusedMailbox begin")

	_, err := d.readMailbox(peerIdHex, reqPubkey, timeout, false)
	if err != nil {
		return err
	}

	if d.lightMailboxUser.ServicePeerID == "" {
		d.lightMailboxUser.ServicePeerID = peerIdHex
	} else if peerIdHex != d.lightMailboxUser.ServicePeerID {
		peerID, err := peer.Decode(peerIdHex)
		if err != nil {
			log.Errorf("MailboxService->releaseUnusedMailbox: fail to decode peer id: %v", err)
			return err
		}
		_, releaseMailboxResponseChan, err := d.releaseMailboxPrtocol.Request(peerID, reqPubkey)
		if err != nil {
			return err
		}
		select {
		case <-releaseMailboxResponseChan:
			log.Debugf("MailboxService->releaseUnusedMailbox: releaseMailboxResponseChan success")
		case <-time.After(time.Second * timeout):
			return fmt.Errorf("MailboxService->releaseUnusedMailbox: releaseMailboxResponseChan time out")
		case <-d.TvBase.GetCtx().Done():
			return fmt.Errorf("MailboxService->releaseUnusedMailbox: BaseService.GetCtx().Done()")
		}
	}

	log.Debugf("MailboxService->releaseUnusedMailbox end")
	return nil
}

func (d *MailboxService) parseReadMailboxResponse(responseProtoData protoreflect.ProtoMessage, direction string) ([]msg.ReceiveMsg, error) {
	log.Debugf("MailboxService->parseReadMailboxResponse begin:\nresponseProtoData: %v", responseProtoData)
	msgList := []msg.ReceiveMsg{}
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

		msgList = append(msgList, msg.ReceiveMsg{
			ID:         msgID,
			ReqPubkey:  srcPubkey,
			DestPubkey: destPubkey,
			Content:    msgContent,
			TimeStamp:  timeStamp,
			Direction:  direction,
		})
	}
	log.Debug("MailboxService->parseReadMailboxResponse end")
	return msgList, nil
}

func (d *MailboxService) saveMsg(protoMsg protoreflect.ProtoMessage) error {
	MsgReq, ok := protoMsg.(*pb.MsgReq)
	if !ok {
		log.Errorf("MailboxService->saveMsg: cannot convert %v to *pb.MsgReq", protoMsg)
		return fmt.Errorf("MailboxService->saveMsg: cannot convert %v to *pb.MsgReq", protoMsg)
	}

	pubkey := MsgReq.BasicData.Pubkey
	user := d.getServiceUser(pubkey)
	if user == nil {
		log.Errorf("MailboxService->saveMsg: cannot find src user pubsub for %v", pubkey)
		return fmt.Errorf("MailboxService->saveMsg: cannot find src user pubsub for %v", pubkey)
	}

	user.MsgRWMutex.RLock()
	defer user.MsgRWMutex.RUnlock()

	key := d.getFullFromMsgPrefix(MsgReq)
	err := d.datastore.Put(d.TvBase.GetCtx(), datastore.NewKey(key), MsgReq.Content)
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

func (d *MailboxService) getFullFromMsgPrefix(MsgReq *pb.MsgReq) string {
	basicPrefix := d.getBasicFromMsgPrefix(MsgReq.BasicData.Pubkey, MsgReq.DestPubkey)
	direction := msg.MsgDirection.From
	return basicPrefix + msg.MsgKeyDelimiter +
		direction + msg.MsgKeyDelimiter +
		MsgReq.BasicData.ID + msg.MsgKeyDelimiter +
		strconv.FormatInt(MsgReq.BasicData.TS, 10)
}
