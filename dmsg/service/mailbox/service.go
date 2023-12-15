package mailbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/tinyverse-web3/tvbase/common/db"
	"github.com/tinyverse-web3/tvbase/common/define"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	dmsgCommonUtil "github.com/tinyverse-web3/tvbase/dmsg/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"

	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/pubsub"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/stream"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/common"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MailboxService struct {
	MailboxBase
	createMailboxProtocol *basic.MailboxSProtocol
	releaseMailboxPrtocol *basic.MailboxSProtocol
	readMailboxMsgPrtocol *basic.MailboxSProtocol
	seekMailboxProtocol   *basic.MailboxPProtocol
	pubsubMsgProtocol     *basic.PubsubMsgProtocol
	serviceUserList       map[string]*dmsgUser.ServiceMailboxUser
	datastore             db.Datastore
	stopCleanRestResource chan bool
	enable                bool
	pubkey                string
}

func NewService(tvbaseService define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) (*MailboxService, error) {
	d := &MailboxService{}
	err := d.Init(tvbaseService, pubkey, getSig)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *MailboxService) Init(tvbaseService define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) error {
	err := d.BaseService.Init(tvbaseService)
	if err != nil {
		return err
	}
	err = d.SubscribeUser(pubkey, getSig)
	if err != nil {
		return err
	}

	cfg := d.BaseService.TvBase.GetConfig()
	filepath := d.BaseService.TvBase.GetRootPath() + cfg.DMsg.DatastorePath
	d.datastore, err = db.CreateBadgerDB(filepath)
	if err != nil {
		log.Errorf("MailboxService->Init: create datastore error %v", err)
		return err
	}

	d.serviceUserList = make(map[string]*dmsgUser.ServiceMailboxUser)
	d.stopCleanRestResource = make(chan bool)
	return nil
}

// sdk-common
func (d *MailboxService) Start() error {
	log.Debugf("MailboxService->Start begin")

	ctx := d.TvBase.GetCtx()
	host := d.TvBase.GetHost()
	// stream protocol
	if d.createMailboxProtocol == nil {
		d.createMailboxProtocol = stream.NewCreateMailboxProtocol(ctx, host, d, d, true, d.pubkey)
	}
	if d.readMailboxMsgPrtocol == nil {
		d.readMailboxMsgPrtocol = stream.NewReadMailboxMsgProtocol(ctx, host, d, d, true, d.pubkey)
	}
	if d.releaseMailboxPrtocol == nil {
		d.releaseMailboxPrtocol = stream.NewReleaseMailboxProtocol(ctx, host, d, d, true, d.pubkey)
	}

	// pubsub protocol
	if d.seekMailboxProtocol == nil {
		d.seekMailboxProtocol = pubsub.NewSeekMailboxProtocol(ctx, host, d, d)
		d.RegistPubsubProtocol(d.seekMailboxProtocol.Adapter.GetRequestPID(), d.seekMailboxProtocol)
	}
	if d.pubsubMsgProtocol == nil {
		d.pubsubMsgProtocol = pubsub.NewPubsubMsgProtocol(ctx, host, d, d)
		d.RegistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetRequestPID(), d.pubsubMsgProtocol)
	}

	//d.cleanRestServiceUser(1 * time.Minute)

	d.enable = true

	// load subscribed user mailbox
	d.loadSubscribeMailboxList()
	log.Debug("MailboxService->Start end")
	return nil
}

func (d *MailboxService) Stop() error {
	log.Debug("MailboxService->Stop begin")

	select {
	case d.stopCleanRestResource <- true:
		log.Debugf("MailboxService->Stop: succ send stopCleanRestResource")
	default:
		log.Debugf("MailboxService->Stop: no receiver for stopCleanRestResource")
	}

	d.enable = false
	log.Debug("MailboxService->Stop end")
	return nil
}

func (d *MailboxService) Release() error {
	err := d.Stop()
	if err != nil {
		return err
	}
	// TODO
	// d.createMailboxProtocol.Release()
	d.createMailboxProtocol = nil
	// d.readMailboxMsgPrtocol.Release()
	d.readMailboxMsgPrtocol = nil
	// d.releaseMailboxPrtocol.Release()
	d.releaseMailboxPrtocol = nil

	d.UnregistPubsubProtocol(d.seekMailboxProtocol.Adapter.GetRequestPID())
	d.seekMailboxProtocol = nil
	d.UnregistPubsubProtocol(d.pubsubMsgProtocol.Adapter.GetRequestPID())
	d.pubsubMsgProtocol = nil

	err = d.UnSubscribeUser()
	if err != nil {
		return err
	}
	err = d.unsubscribeServiceUserList()
	if err != nil {
		return err
	}
	if d.datastore != nil {
		d.datastore.Close()
		d.datastore = nil
	}

	close(d.stopCleanRestResource)
	return nil
}

// DmsgService

func (d *MailboxService) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	var target *dmsgUser.Target
	user := d.serviceUserList[pubkey]
	if user != nil {
		target = &user.Target
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
func (d *MailboxService) OnCreateMailboxRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	if !d.enable {
		return nil, nil, true, nil
	}
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

	log.Infof("MailboxService->OnCreateMailboxRequest: Create mail box: %s", pubkey)

	user := d.getServiceUser(pubkey)
	if user != nil {
		log.Errorf("MailboxService->OnCreateMailboxRequest: pubkey is already exist in serviceUserList")
		retCode := &pb.RetCode{
			Code:   common.AlreadyExistCode,
			Result: "MailboxService->OnCreateMailboxRequest: pubkey already exist in serviceUserList",
		}
		return nil, retCode, false, nil
	}

	err := d.subscribeServiceUser(pubkey)
	if err != nil {
		return nil, nil, false, err
	}

	// Have new user create mailbox, save it
	d.saveSubscribeMailboxList()

	log.Debugf("MailboxService->OnCreateMailboxRequest end")
	return nil, nil, false, nil
}

func (d *MailboxService) OnReleaseMailboxRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("MailboxService->OnReleaseMailboxRequest begin:\nrequestProtoData: %+v", requestProtoData)
	if !d.enable {
		return nil, nil, true, nil
	}
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

	// Have user delete mailbox subscribed, save it
	d.saveSubscribeMailboxList()

	log.Debugf("MailboxService->OnReleaseMailboxRequest end")
	return nil, nil, false, nil
}

func (d *MailboxService) OnReadMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("MailboxService->OnReadMailboxRequest begin:\nrequestProtoData: %+v", requestProtoData)
	if !d.enable {
		return nil, nil, true, nil
	}
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
	log.Infof("MailboxService->OnReadMailboxRequest: Read mail box user : %s", pubkey)
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
		log.Errorf("MailboxService->OnReadMailboxRequest: Read mail box  : No mail for %s", pubkey)
		return nil, nil, false, err
	}
	defer results.Close()

	mailboxMsgDataList := []*pb.MailboxItem{}
	find := false
	needDeleteKeyList := []string{}
	const MaxContentSize = 2 * 1024 * 1024
	factSize := 0
	requestParam := &stream.ReadMailRequestParam{}
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

// MailboxPpCallback
func (d *MailboxService) OnSeekMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("MailboxService->OnSeekMailboxRequest begin\nrequestProtoData: %+v", requestProtoData)
	if !d.enable {
		log.Errorf("MailboxService->OnSeekMailboxRequest: fail to d.enable is false")
		return nil, nil, true, nil
	}
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
	if request.BasicData.ProxyPubkey != "" {
		pubkey = request.BasicData.ProxyPubkey
	}

	user := d.getServiceUser(pubkey)
	if user == nil {
		log.Errorf("MailboxService->OnSeekMailboxRequest: cannot find user for pubkey: %s", pubkey)
		retCode := &pb.RetCode{
			Code:   common.NoExistCode,
			Result: "MailboxService->OnSeekMailboxRequest: pubkey no exist in serviceUserList",
		}
		return nil, retCode, false, fmt.Errorf("MailboxService->OnSeekMailboxRequest: cannot find user for pubkey: %s", pubkey)
	}

	log.Debug("MailboxService->OnSeekMailboxRequest end")
	return nil, nil, false, nil
}

func (d *MailboxService) OnPubsubMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	log.Debugf("MailboxService->OnPubsubMsgRequest begin:\nrequestProtoData: %+v", requestProtoData)
	if !d.enable {
		return nil, nil, true, nil
	}
	request, ok := requestProtoData.(*pb.MsgReq)
	if !ok {
		log.Errorf("MailboxService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
		return nil, nil, true, fmt.Errorf("MailboxService->OnPubsubMsgRequest: fail to convert requestProtoData to *pb.MsgReq")
	}

	/*
		if request.BasicData.PeerID == d.TvBase.GetHost().ID().String() {
			log.Debugf("MailboxService->OnCreatePubusubRequest: request.BasicData.PeerID == d.TvBase.GetHost().ID().String()")
			return nil, nil, true, nil
		}
	*/

	/*
		pubkey := request.BasicData.Pubkey
		if request.BasicData.ProxyPubkey != "" {
			pubkey = request.BasicData.ProxyPubkey
		}
	*/
	pubkey := request.DestPubkey

	log.Infof("MailboxService->OnPubsubMsgRequest: Receive msg with %s", pubkey)

	user := d.getServiceUser(pubkey)
	if user == nil {
		log.Errorf("MailboxService->OnPubsubMsgRequest: public key %s is not exist", pubkey)
		return nil, nil, true, fmt.Errorf("MailboxService->OnPubsubMsgRequest: public key %s is not exist", pubkey)
	}

	user.MsgRWMutex.RLock()
	defer user.MsgRWMutex.RUnlock()

	key := d.getFullFromMsgPrefix(request)
	err := d.datastore.Put(d.TvBase.GetCtx(), datastore.NewKey(key), request.Content)

	if err != nil {
		log.Errorf("MailboxService->OnPubsubMsgRequest: fail to save msg: %s", err.Error())
		return nil, nil, true, fmt.Errorf("MailboxService->OnPubsubMsgRequest: fail to save msg: %s", err.Error())
	}

	user.LastTimestamp = time.Now().UnixNano()

	log.Debugf("MailboxService->OnPubsubMsgRequest end")
	return nil, nil, true, nil
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

func (d *MailboxService) UnSubscribeUser() error {
	log.Debugf("MailboxService->UnSubscribeUser begin")
	if d.lightMailboxUser == nil {
		log.Errorf("MailboxService->UnSubscribeUser: userPubkey is not exist in destUserInfoList")
		return fmt.Errorf("MailboxService->UnSubscribeUser: userPubkey is not exist in destUserInfoList")
	}
	d.lightMailboxUser.Close()
	d.lightMailboxUser = nil
	log.Debugf("MailboxService->UnSubscribeUser end")
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

func (d *MailboxService) getMsgPrefix(pubkey string) string {
	return msg.MsgPrefix + pubkey
}

func (d *MailboxService) getBasicFromMsgPrefix(srcUserPubkey string, destUserPubkey string) string {
	return msg.MsgPrefix + destUserPubkey + msg.MsgKeyDelimiter + srcUserPubkey
}

func (d *MailboxService) getFullFromMsgPrefix(request *pb.MsgReq) string {
	basicPrefix := d.getBasicFromMsgPrefix(request.BasicData.Pubkey, request.DestPubkey)
	direction := msg.MsgDirection.From
	return basicPrefix + msg.MsgKeyDelimiter +
		direction + msg.MsgKeyDelimiter +
		request.BasicData.ID + msg.MsgKeyDelimiter +
		strconv.FormatInt(request.BasicData.TS, 10)
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

type MailboxSubscribeList struct {
	UserList []string `json:"user_list"`
}

const SubscribeMailboxListPath = "subscribe_mailbox_list.json"
const SubscribeMailboxListDBName = "subscribe_mailbox_list"

func (d *MailboxService) saveSubscribeMailboxList1() {

	log.Debugf("MailboxService->saveSubscribeMailboxList: begin")
	subscribeUserList := &MailboxSubscribeList{
		UserList: make([]string, 0),
	}

	for pubkey := range d.serviceUserList {
		subscribeUserList.UserList = append(subscribeUserList.UserList, pubkey)
	}

	subscribeUserListByte, err := json.Marshal(subscribeUserList)
	if err != nil {
		log.Errorf("MailboxService->saveSubscribeMailboxList: json.Marshal error: %v", err)
		return
	}

	path := d.TvBase.GetRootPath() + SubscribeMailboxListPath
	err = os.WriteFile(path, subscribeUserListByte, 0644)
	if err != nil {
		log.Errorf("MailboxService->saveSubscribeMailboxList: os.WriteFile error: %v", err)
		return
	}

	log.Infof("MailboxService->saveSubscribeMailboxList: list <%v> has been saved: %s", subscribeUserList.UserList, path)
}

func (d *MailboxService) saveSubscribeMailboxList() {
	log.Debugf("saveSubscribeMailboxList...\n")
	// 打开已有的 LevelDB 数据库
	options := &opt.Options{
		// 如果数据库不存在，自动创建数据库
		ErrorIfMissing: false,
	}
	var err error
	//reserveFilename := rootPath + "reservename"
	dbName_SubscribedList := d.TvBase.GetRootPath() + SubscribeMailboxListDBName
	dbSubscribedList, err := leveldb.OpenFile(dbName_SubscribedList, options)
	if err != nil {
		log.Errorf("Open DB subscribedList failed, %v.\n", err)
		return
	}
	defer dbSubscribedList.Close()

	log.Debugf("The DBs have been opened...\n")
	// 设置迭代器范围为整个数据库
	iter := dbSubscribedList.NewIterator(nil, nil)
	defer iter.Release()

	// Clear data
	for iter.Next() {
		userid := string(iter.Key())
		value := iter.Value()
		if d.serviceUserList[userid] != nil {
			// The key is subscribed mailbox
			if string(value) != "1" {
				// The value is unsubscribe, restore the key
				log.Debugf("Restore user <%s> to db.", userid)
				value = []byte("1")
				err = dbSubscribedList.Put([]byte(userid), value, nil)
				if err != nil {
					log.Errorf("Put DB subscribedList failed, %v.\n", err)
				}
			}
		} else {
			// The user is not subscribed mailbox, clear the key
			if string(value) != "0" {
				// The value is subscribed, clear the key
				log.Debugf("Clear user <%s> to db.", userid)
				value = []byte("0")
				err = dbSubscribedList.Put([]byte(userid), value, nil)
				if err != nil {
					log.Errorf("Put DB subscribedList failed, %v.\n", err)
				}
			}
		}
	}

	// check all users is recorded
	for pubkey := range d.serviceUserList {
		_, err := dbSubscribedList.Get([]byte(pubkey), nil)
		if err != nil {
			// The user is not record it, put it into db
			log.Debugf("Add new user <%s> to db.", pubkey)
			value := []byte("1")
			err = dbSubscribedList.Put([]byte(pubkey), value, nil)
			if err != nil {
				log.Errorf("Put DB subscribedList failed, %v.\n", err)
			}
		}

	}

	log.Debugf("saveSubscribeMailboxList Done.")

}

func (d *MailboxService) loadJsonSubscribeMailboxList() *MailboxSubscribeList {
	log.Debugf("MailboxService->loadSubscribeMailboxList: begin")
	subscribeUserList := &MailboxSubscribeList{}

	path := d.TvBase.GetRootPath() + SubscribeMailboxListPath
	subscribeUserListByte, err := os.ReadFile(path)
	if err != nil {
		log.Errorf("MailboxService->loadSubscribeMailboxList: read error: %v", err)
		return nil
	}

	err = json.Unmarshal(subscribeUserListByte, subscribeUserList)
	if err != nil {
		log.Errorf("MailboxService->saveSubscribeMailboxList: json.Unmarshal error: %v", err)
		return nil
	}

	// The data has been loaded, remove it.
	os.Remove(path)

	log.Infof("MailboxService->saveSubscribeMailboxList: list <%v> has been loaded", subscribeUserList.UserList)

	return subscribeUserList
	/*
		for _, pubkey := range subscribeUserList.UserList {
			user := d.getServiceUser(pubkey)
			if user != nil {
				log.Errorf("MailboxService->OnCreateMailboxRequest: pubkey is already exist in serviceUserList")
				continue
			}

			err := d.subscribeServiceUser(pubkey)
			if err != nil {
				continue
			}
		}

		log.Debugf("MailboxService->loadSubscribeMailboxList: end")
	*/
}

func (d *MailboxService) loadSubscribeMailboxList() {
	log.Debugf("loadSubscribeMailboxList...")
	// 打开已有的 LevelDB 数据库
	options := &opt.Options{
		// 如果数据库不存在，自动创建数据库
		ErrorIfMissing: false,
	}
	var err error
	//reserveFilename := rootPath + "reservename"
	dbName_SubscribedList := d.TvBase.GetRootPath() + SubscribeMailboxListDBName
	dbSubscribedList, err := leveldb.OpenFile(dbName_SubscribedList, options)
	if err != nil {
		log.Errorf("Open DB subscribedList failed, %v.\n", err)
		return
	}
	defer dbSubscribedList.Close()

	log.Debugf("The DBs have been opened...\n")

	jsonSubscribeMailboxList := d.loadJsonSubscribeMailboxList()
	if jsonSubscribeMailboxList != nil {
		for _, userid := range jsonSubscribeMailboxList.UserList {
			value := []byte("1")
			err = dbSubscribedList.Put([]byte(userid), value, nil)
			if err != nil {
				log.Errorf("Put DB subscribedList failed, %v.\n", err)
			}
		}
	}

	// 设置迭代器范围为整个数据库
	iter := dbSubscribedList.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		userid := string(iter.Key())
		value := iter.Value()

		if string(value) == "1" {
			// The value is recored with subscribed, subscribe to pubsub
			user := d.getServiceUser(userid)
			if user != nil {
				log.Errorf("MailboxService->OnCreateMailboxRequest: pubkey is already exist in serviceUserList")
				continue
			}

			log.Debugf("Subscribe the user <%s>.", userid)
			err := d.subscribeServiceUser(userid)
			if err != nil {
				continue
			}
		}

	}
	log.Debugf("loadSubscribeMailboxList Done.\n")

}
