package pullcid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/crypto"
	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	tvIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	"github.com/tinyverse-web3/tvbase/dkvs"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
)

const LoggerName = "dmsg.protocol.custom.pullcid"

var log = ipfsLog.Logger(LoggerName)

const pullCidPID = "pullcid"
const StorageKeyPrefix = "/storage012345678901234567890123456789/ipfs012345678901234567890123456789/"

type serviceCommicateInfo struct {
	data any
}

type PullCidRequest struct {
	CID                 string
	MaxCheckTime        time.Duration
	StorageProviderList []string
}

type PullCidResponse struct {
	CID            string
	CidContentSize int64
	ElapsedTime    time.Duration
	Status         tvIpfs.PidStatus
}

// client
type PullCidClientProtocol struct {
	customProtocol.CustomStreamClientProtocol
}

var pullCidClientProtocol *PullCidClientProtocol
var pullCidServiceProtocol *PullCidServiceProtocol

func GetPullCidClientProtocol() (*PullCidClientProtocol, error) {
	if pullCidClientProtocol == nil {
		pullCidClientProtocol = &PullCidClientProtocol{}
		err := pullCidClientProtocol.Init()
		if err != nil {
			return nil, err
		}
	}
	return pullCidClientProtocol, nil
}

func (p *PullCidClientProtocol) Init() error {
	p.CustomStreamClientProtocol.Init(pullCidPID)
	return nil
}

func (p *PullCidClientProtocol) Request(peerId string, pullcidRequest *PullCidRequest) (chan *PullCidResponse, error) {
	log.Debugf("PullCidClientProtocol->Request begin:\npeerId: %s \nrequest: %v", peerId, pullcidRequest)
	_, err := cid.Decode(pullcidRequest.CID)
	if err != nil {
		log.Errorf("PullCidClientProtocol->Request: cid.Decode: err: %v, cid: %s", err, pullcidRequest.CID)
		return nil, err
	}

	_, customProtocolRespChan, err := p.CustomStreamClientProtocol.Request(peerId, pullcidRequest)
	if err != nil {
		log.Errorf("PullCidClientProtocol->Request: err: %v", err)
		return nil, err
	}

	ret := make(chan *PullCidResponse)
	go func() {
		select {
		case data := <-customProtocolRespChan:
			customProtocolResponse, ok := data.(*pb.CustomProtocolRes)
			if !ok {
				log.Errorf("PullCidClientProtocol->Request: responseChan is not CustomProtocolRes")
				ret <- nil
				return
			}
			response := &PullCidResponse{}
			err := p.CustomStreamClientProtocol.HandleResponse(customProtocolResponse, response)
			if err != nil {
				log.Errorf("PullCidClientProtocol->Request: CustomStreamClientProtocol.HandleResponse error: %v", err)
				ret <- nil
				return
			}
			ret <- response
			return
		case <-p.Ctx.Done():
			ret <- nil
			return
		}
	}()
	return ret, nil
}

// service
type PullCidServiceProtocol struct {
	PriKey crypto.PrivKey
	customProtocol.CustomStreamServiceProtocol
	commicateInfoList      map[string]*serviceCommicateInfo
	commicateInfoListMutex sync.Mutex
	tvBaseService          tvbaseCommon.TvBaseService
	ipfsProviderList       map[string]*ipfsProviderList
	storageInfoList        *map[string]any
}

type ipfsUpload func(providerName string, cid string) error

type ipfsProviderList struct {
	uploadTaskList map[string]*uploadTask
	mutex          sync.Mutex
	interval       time.Duration
	timeout        time.Duration
	uploadUrl      string
	apiKey         string
	timer          *time.Ticker
	taskQueue      chan bool
	uploadFunc     ipfsUpload
	isRun          bool
}

type uploadTask struct {
	cid string
}

var (
	NftProvider = "nft"
	NftApiKey   = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkaWQ6ZXRocjoweDgxOTgwNzg4Y2UxQjY3MDQyM2Y1NzAyMDQ2OWM0MzI3YzNBNzU5YzciLCJpc3MiOiJuZnQtc3RvcmFnZSIsImlhdCI6MTY5MDUyODU1MjcxMywibmFtZSI6InRlc3QxIn0.vslsn8tAWUtZ0BZjcxhyMrcuufwfZ7fTMpF_DrojF4c"
	NftPostURL  = "https://api.nft.storage/upload"

	Web3Provider = "web3"
	Web3ApiKey   = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkaWQ6ZXRocjoweDAyYzZEYkJBMTQyOTA1MzliZjgwNkEzRkNDRDgzMDFmNWNjNTQ2ZDIiLCJpc3MiOiJ3ZWIzLXN0b3JhZ2UiLCJpYXQiOjE2OTA2ODkxMjg3NDUsIm5hbWUiOiJ0ZXN0In0.nhArwLJYjFwTiW1-SSRPyrCCczyYQ4T2PAHcShFZXqg"
	Web3PostURL  = "https://api.web3.storage/upload"

	UploadInterval = 3 * time.Second
	UploadTimeout  = 30 * time.Second
)

func GetPullCidServiceProtocol(tvBaseService tvbaseCommon.TvBaseService) (*PullCidServiceProtocol, error) {
	if pullCidServiceProtocol == nil {
		pullCidServiceProtocol = &PullCidServiceProtocol{}
		err := pullCidServiceProtocol.Init(tvBaseService)
		if err != nil {
			return nil, err
		}
	}
	return pullCidServiceProtocol, nil
}

func (p *PullCidServiceProtocol) Init(tvBaseService tvbaseCommon.TvBaseService) error {
	if p.PriKey == nil {
		prikey, err := dkvs.GetPriKeyBySeed(pullCidPID)
		if err != nil {
			log.Errorf("PullCidServiceProtocol->Init: GetPriKeyBySeed err: %v", err)
			return err
		}
		p.PriKey = prikey

	}
	p.CustomStreamServiceProtocol.Init(pullCidPID)
	p.tvBaseService = tvBaseService
	p.commicateInfoList = make(map[string]*serviceCommicateInfo)
	p.storageInfoList = &map[string]any{}
	p.ipfsProviderList = make(map[string]*ipfsProviderList)
	p.initIpfsProviderTask(NftProvider, NftApiKey, NftPostURL, UploadInterval, UploadTimeout, p.httpUploadCidContent)
	p.initIpfsProviderTask(Web3Provider, Web3ApiKey, Web3PostURL, UploadInterval, UploadTimeout, p.httpUploadCidContent)
	return nil
}

func (p *PullCidServiceProtocol) HandleRequest(request *pb.CustomProtocolReq) error {
	log.Debugf("PullCidServiceProtocol->HandleRequest begin:\nrequest: %v", request)
	pullCidRequest := &PullCidRequest{}
	err := p.CustomStreamServiceProtocol.HandleRequest(request, pullCidRequest)
	if err != nil {
		return err
	}
	err = tvIpfs.CheckIpfsCmd()
	if err != nil {
		return err
	}
	if p.commicateInfoList[pullCidRequest.CID] != nil {
		return nil
	}

	maxCheckTime := pullCidRequest.MaxCheckTime
	if maxCheckTime <= 0 {
		// default timeout is 3 minute
		maxCheckTime = 3 * time.Minute
	} else if maxCheckTime < 10*time.Second {
		// min timeout is 10 second
		maxCheckTime = 10 * time.Second
	} else if maxCheckTime > 3*time.Hour {
		// max timeout is 3 hour
		maxCheckTime = 3 * time.Hour
	}

	timer := time.NewTimer(500 * time.Millisecond)
	go func() {
		cidContentSize, elapsedTime, pinStatus, err := tvIpfs.IpfsGetObject(pullCidRequest.CID, p.Ctx, maxCheckTime)
		if err != nil {
			log.Errorf("PullCidServiceProtocol->HandleRequest: err: %v", err)
			return
		}

		pullCidResponse := &PullCidResponse{
			CID:            pullCidRequest.CID,
			CidContentSize: cidContentSize,
			ElapsedTime:    elapsedTime,
			Status:         pinStatus,
		}
		p.commicateInfoListMutex.Lock()
		defer p.commicateInfoListMutex.Unlock()
		p.commicateInfoList[pullCidRequest.CID] = &serviceCommicateInfo{
			data: pullCidResponse,
		}
		log.Debugf("PullCidServiceProtocol->HandleRequest: cid: %v, pullCidResponse: %v",
			pullCidRequest.CID, pullCidResponse)
	}()

	<-timer.C
	log.Debugf("PullCidServiceProtocol->HandleRequest end")
	return nil
}

func (p *PullCidServiceProtocol) HandleResponse(request *pb.CustomProtocolReq, response *pb.CustomProtocolRes) error {
	log.Debugf("PullCidServiceProtocol->HandleResponse begin:\nrequest: %v \nresponse: %v", request, response)
	pullCidRequest := &PullCidRequest{}
	err := p.CustomStreamServiceProtocol.HandleRequest(request, pullCidRequest)
	if err != nil {
		log.Errorf("PullCidServiceProtocol->HandleResponse: err: %v", err)
		return err
	}

	commicateInfo := p.commicateInfoList[pullCidRequest.CID]
	if commicateInfo == nil {
		log.Debugf("PullCidServiceProtocol->HandleResponse: need wait requestHandle handle cid: %s", pullCidRequest.CID)
		return fmt.Errorf("PullCidServiceProtocol->HandleResponse: need wait requestHandle handle cid: %s", pullCidRequest.CID)
	}

	pullCidResponse, ok := commicateInfo.data.(*PullCidResponse)
	if !ok {
		log.Infof("PullCidServiceProtocol->HandleResponse: pullCidResponse is nil, cid: %s", pullCidResponse.CID)
		return fmt.Errorf("PullCidServiceProtocol->HandleResponse: pullCidResponse is nil, cid: %s", pullCidRequest.CID)
	}

	err = p.CustomStreamServiceProtocol.HandleResponse(response, pullCidResponse)
	if err != nil {
		log.Errorf("PullCidServiceProtocol->HandleResponse: err: %v", err)
		return err
	}

	switch pullCidResponse.Status {
	case tvIpfs.PinStatus_ERR, tvIpfs.PinStatus_PINNED, tvIpfs.PinStatus_TIMEOUT:
		delete(p.commicateInfoList, pullCidRequest.CID)
	default:
		log.Debugf("PullCidServiceProtocol->HandleResponse: cid: %v, pullCidResponse: %v, status: %v, pullcid working....",
			pullCidRequest.CID, pullCidResponse, pullCidResponse.Status)
	}

	if pullCidResponse.Status == tvIpfs.PinStatus_PINNED {
		err = p.uploadContentToProvider(pullCidResponse.CID, pullCidRequest.StorageProviderList)
		if err != nil {
			return err
		}
		err = p.saveCidInfoToDkvs(pullCidResponse.CID)
		if err != nil {
			return err
		}
	}
	log.Debugf("PullCidServiceProtocol->HandleResponse end")
	return nil
}

func (p *PullCidServiceProtocol) uploadContentToProvider(cid string, storageProviderList []string) error {
	log.Debugf("PullCidServiceProtocol->uploadContentToProvider begin: \ncid:%s\nstorageProviderList:%v",
		cid, storageProviderList)
	for _, storageProvider := range storageProviderList {
		(*p.storageInfoList)[storageProvider] = storageProvider
		p.asyncUploadCidContent(storageProvider, cid)
	}
	log.Debugf("PullCidServiceProtocol->uploadContentToProvider end")
	return nil
}

func (p *PullCidServiceProtocol) saveCidInfoToDkvs(cid string) error {
	log.Debugf("PullCidServiceProtocol->saveCidInfoToDkvs begin: cid:%s", cid)
	dkvsKey := StorageKeyPrefix + cid
	pubkeyData, err := crypto.MarshalPublicKey(p.PriKey.GetPublic())
	if err != nil {
		log.Errorf("PullCidServiceProtocol->saveCidInfoToDkvs: crypto.MarshalPublicKey error: %v", err)
		return err
	}
	peerID := p.tvBaseService.GetHost().ID().String()
	isExistKey := p.tvBaseService.GetDkvsService().Has(dkvsKey)
	if isExistKey {
		value, _, _, _, _, err := p.tvBaseService.GetDkvsService().Get(dkvsKey)
		if err != nil {
			log.Errorf("PullCidServiceProtocol->saveCidInfoToDkvs: GetDkvsService->Get error: %v", err)
			return err
		}
		err = json.Unmarshal(value, p.storageInfoList)
		if err != nil {
			log.Warnf("PullCidServiceProtocol->saveCidInfoToDkvs: json.Unmarshal old dkvs value error: %v", err)
			return nil
		}
		if (*p.storageInfoList)[peerID] != nil {
			log.Debugf("PullCidServiceProtocol->saveCidInfoToDkvs: peerID is already exist in dkvs, peerID: %s", peerID)
			return nil
		}
	}

	(*p.storageInfoList)[peerID] = peerID
	value, err := json.Marshal(p.storageInfoList)
	if err != nil {
		log.Errorf("PullCidServiceProtocol->saveCidInfoToDkvs: json marshal new dkvs value error: %v", err)
		return err
	}
	issuetime := dkvs.TimeNow()
	ttl := dkvs.GetTtlFromDuration(time.Hour * 24 * 30 * 12 * 100) // about 100 year
	sig, err := p.PriKey.Sign(dkvs.GetRecordSignData(dkvsKey, value, pubkeyData, issuetime, ttl))
	if err != nil {
		log.Errorf("PullCidServiceProtocol->saveCidInfoToDkvs: SignDataByEcdsa: %v", err)
		return err
	}

	err = p.tvBaseService.GetDkvsService().Put(dkvsKey, value, pubkeyData, issuetime, ttl, sig)
	if err != nil {
		log.Errorf("PullCidServiceProtocol->saveCidInfoToDkvs: Put error: %v", err)
		return err
	}
	log.Debugf("PullCidServiceProtocol->saveCidInfoToDkvs end")
	return nil
}

func (p *PullCidServiceProtocol) httpUploadCidContent(providerName string, cid string) error {
	// the same API key exceeds 30 request within 10 seconds, the rate limit will be triggered
	// https://nft.storage/api-docs/  https://web3.storage/docs/reference/http-api/
	log.Debugf("PullCidServiceProtocol->httpUploadCidContent begin: providerName:%s, cid: %s", providerName, cid)
	provider := p.ipfsProviderList[providerName]
	if provider == nil {
		log.Errorf("PullCidServiceProtocol->httpUploadCidContent: provider is nil, providerName: %s", providerName)
		return fmt.Errorf("PullCidServiceProtocol->httpUploadCidContent: provider is nil, providerName: %s", providerName)
	}

	timeoutCtx, cancel := context.WithTimeout(p.Ctx, 10*time.Second)
	defer cancel()
	content, _, err := tvIpfs.IpfsBlockGet(cid, timeoutCtx)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(content)
	bufSize := len(buf.Bytes())
	if bufSize >= 100*1024*1024 {
		// TODO over 100MB need to be split using CAR,, implement it
		log.Errorf("PullCidServiceProtocol->httpUploadCidContent: file too large(<100MB), bufSize:%v", bufSize)
		return fmt.Errorf("PullCidServiceProtocol->httpUploadCidContent: file too large(<100MB), bufSize:%v", bufSize)
	}

	client := &http.Client{
		Timeout: provider.timeout,
	}
	req, err := http.NewRequest("POST", provider.uploadUrl, buf)
	if err != nil {
		log.Errorf("PullCidServiceProtocol->httpUploadCidContent: http.NewRequest error: %v", err)
		return nil
	}

	req.Header.Set("Authorization", "Bearer "+provider.apiKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("PullCidServiceProtocol->httpUploadCidContent: client.Do error: %v", err)
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("PullCidServiceProtocol->httpUploadCidContent: ioutil.ReadAll error: %v", err)
		return err
	}

	log.Debugf("PullCidServiceProtocol->httpUploadCidContent: body: %s", string(responseBody))
	log.Debugf("PullCidServiceProtocol->httpUploadCidContent end")
	return nil
}

func (p *PullCidServiceProtocol) initIpfsProviderTask(providerName string, apiKey string, uploadUrl string, interval time.Duration, timeout time.Duration, uploadFunc ipfsUpload) error {
	if p.ipfsProviderList[providerName] != nil {
		log.Errorf("PullCidServiceProtocol->initIpfsProviderTask: ipfsProviderTaskList[providerName] is not nil")
		return fmt.Errorf("PullCidServiceProtocol->initIpfsProviderTask: ipfsProviderTaskList[providerName] is not nil")
	}
	p.ipfsProviderList[providerName] = &ipfsProviderList{
		uploadTaskList: make(map[string]*uploadTask),
		interval:       interval,
		timeout:        timeout,
		uploadUrl:      uploadUrl,
		apiKey:         apiKey,
		uploadFunc:     uploadFunc,
		isRun:          false,
	}
	return nil
}

func (p *PullCidServiceProtocol) asyncUploadCidContent(providerName string, cid string) error {
	log.Debugf("PullCidServiceProtocol->asyncUploadCidContent begin:\nproviderName:%s\n cid: %v",
		providerName, cid)
	dkvsKey := StorageKeyPrefix + cid
	isExistKey := p.tvBaseService.GetDkvsService().Has(dkvsKey)
	if isExistKey {
		value, _, _, _, _, err := p.tvBaseService.GetDkvsService().Get(dkvsKey)
		if err != nil {
			log.Errorf("PullCidServiceProtocol->asyncUploadCidContent: GetDkvsService->Get error: %v", err)
			return err
		}
		err = json.Unmarshal(value, p.storageInfoList)
		if err != nil {
			log.Warnf("PullCidServiceProtocol->asyncUploadCidContent: json.Unmarshal old dkvs value error: %v", err)
			return err
		}
		if (*p.storageInfoList)[providerName] != nil {
			log.Debugf("PullCidServiceProtocol->asyncUploadCidContent: provider is already exist in dkvs, providerName: %s", providerName)
			return nil
		}
	}

	ipfsProviderTask := p.ipfsProviderList[providerName]
	if ipfsProviderTask == nil {
		log.Errorf("PullCidServiceProtocol->asyncUploadCidContent: ipfsProviderList[providerName] is nil")
		return fmt.Errorf("PullCidServiceProtocol->asyncUploadCidContent: ipfsProviderList[providerName] is nil")
	}
	ipfsProviderTask.mutex.Lock()
	ipfsProviderTask.uploadTaskList[cid] = &uploadTask{
		cid: cid,
	}
	ipfsProviderTask.mutex.Unlock()
	if ipfsProviderTask.isRun {
		return nil
	}

	ipfsProviderTask.isRun = true
	ipfsProviderTask.taskQueue = make(chan bool)
	defer close(ipfsProviderTask.taskQueue)
	done := make(chan bool)
	defer close(done)
	if ipfsProviderTask.timer == nil {
		ipfsProviderTask.timer = time.NewTicker(ipfsProviderTask.timeout)
	}
	ipfsProviderTask.timer.Reset(ipfsProviderTask.interval)
	go func() {
		for {
			select {
			case <-ipfsProviderTask.taskQueue:
				if (len(ipfsProviderTask.uploadTaskList)) > 0 {
					ipfsProviderTask.mutex.Lock()
					for _, uploadTask := range ipfsProviderTask.uploadTaskList {
						delete(ipfsProviderTask.uploadTaskList, uploadTask.cid)
						ipfsProviderTask.uploadFunc(providerName, uploadTask.cid)
						break
					}
					ipfsProviderTask.mutex.Unlock()
				} else {
					done <- true
					return
				}
			case <-p.Ctx.Done():
				done <- true
				return
			}
		}
	}()

	for {
		select {
		case <-ipfsProviderTask.timer.C:
			ipfsProviderTask.taskQueue <- true
			log.Debugf("PullCidServiceProtocol->asyncUploadCidContent: ipfsProviderTask.timer.C")
		case <-done:
			ipfsProviderTask.timer.Stop()
			ipfsProviderTask.isRun = false
			log.Debugf("PullCidServiceProtocol->asyncUploadCidContent end: done")
			return nil
		case <-p.Ctx.Done():
			ipfsProviderTask.timer.Stop()
			ipfsProviderTask.isRun = false
			log.Debugf("PullCidServiceProtocol->asyncUploadCidContent end: p.Ctx.Done()")
			return p.Ctx.Err()
		}
	}
}
