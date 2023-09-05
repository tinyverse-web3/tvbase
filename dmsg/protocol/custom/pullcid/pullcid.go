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
	cmap "github.com/orcaman/concurrent-map"
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
	resp               *PullCidResponse
	done               chan any
	concurrentReqCount int
	concurrentReqMutex *sync.Mutex
}

type PullCidRequest struct {
	CID                 string
	MaxCheckTime        time.Duration
	StorageProviderList []string
}

type ReqStatus int

const (
	ReqStatus_WORK ReqStatus = iota
	ReqStatus_ERR
	ReqStatus_TIMEOUT
	ReqStatus_FINISH
)

type PullCidResponse struct {
	CID         string
	ContentSize int64
	ElapsedTime time.Duration
	PinStatus   tvIpfs.PidStatus
	Status      ReqStatus
	Result      string
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

func (p *PullCidClientProtocol) Request(ctx context.Context, peerId string, pullcidRequest *PullCidRequest) (chan *PullCidResponse, error) {
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
		case <-ctx.Done():
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
	commicateInfoList      cmap.ConcurrentMap
	commicateInfoListMutex sync.RWMutex
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
	
	m := cmap.New()
	err := tvIpfs.CheckIpfsCmd()
	if err != nil {
		return err
	}
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
	log.Debugf("PullCidServiceProtocol->HandleRequest: pullCidRequest: %v", pullCidRequest)

	const maxReqCount = 1000
	checkReqCount := func() bool {
		p.commicateInfoListMutex.Lock()
		defer p.commicateInfoListMutex.Unlock()
		p.commicateInfoList.
		if len(p.commicateInfoList) > maxReqCount {
			info := &serviceCommicateInfo{
				resp: &PullCidResponse{
					CID:         pullCidRequest.CID,
					ContentSize: 0,
					ElapsedTime: 0,
					PinStatus:   tvIpfs.PinStatus_UNKNOW,
					Status:      ReqStatus_ERR,
					Result:      "too many request",
				},
			}
			p.commicateInfoList[pullCidRequest.CID] = info
			return false
		}
		return true
	}
	if checkReqCount() {
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

	p.commicateInfoListMutex.RLock()
	info := p.commicateInfoList[pullCidRequest.CID]
	p.commicateInfoListMutex.RUnlock()

	if info != nil {
		info.concurrentReqMutex.Lock()
		info.concurrentReqCount++
		info.concurrentReqMutex.Unlock()
		select {
		case <-info.done:
		case <-time.After(maxCheckTime):
			info.concurrentReqMutex.Lock()
			info.concurrentReqCount--
			info.concurrentReqMutex.Unlock()
		}
		return nil
	}

	info = &serviceCommicateInfo{
		resp: &PullCidResponse{
			CID:         pullCidRequest.CID,
			ContentSize: 0,
			ElapsedTime: 0,
			PinStatus:   tvIpfs.PinStatus_UNKNOW,
			Status:      ReqStatus_WORK,
			Result:      "",
		},
		done:               make(chan any),
		concurrentReqCount: 1,
		concurrentReqMutex: &sync.Mutex{},
	}

	p.commicateInfoListMutex.Lock()
	p.commicateInfoList[pullCidRequest.CID] = info
	p.commicateInfoListMutex.Unlock()

	execTask := func() {
		contentSize, elapsedTime, pinStatus, err := tvIpfs.IpfsGetObject(pullCidRequest.CID, p.Ctx, maxCheckTime)
		info.resp.ContentSize = contentSize
		info.resp.ElapsedTime = elapsedTime
		info.resp.PinStatus = pinStatus

		switch info.resp.PinStatus {
		case tvIpfs.PinStatus_ERR:
			log.Errorf("PullCidServiceProtocol->HandleRequest: tvIpfs.IpfsGetObject error: %v", err)
			info.resp.Status = ReqStatus_ERR
			if err != nil {
				info.resp.Result = err.Error()
			} else {
				info.resp.Result = "unknow ipfs error"
			}
		case tvIpfs.PinStatus_TIMEOUT:
			info.resp.Status = ReqStatus_TIMEOUT
			info.resp.Result = "error ipfs pin status: timeout"
		case tvIpfs.PinStatus_PINNED, tvIpfs.PinStatus_ALREADY_PINNED:
			info.resp.Status = ReqStatus_FINISH
			info.resp.Result = "success"
		case tvIpfs.PinStatus_UNKNOW:
			// never happen
			log.Error("PullCidServiceProtocol->HandleRequest: tvIpfs.IpfsGetObject Unknow")
			info.resp.Status = ReqStatus_ERR
			info.resp.Result = "error ipfs pin status: unknown"
		case tvIpfs.PinStatus_INIT:
			// never happen
			log.Debug("PullCidServiceProtocol->HandleRequest: tvIpfs.IpfsGetObject Init")
			info.resp.Status = ReqStatus_ERR
			info.resp.Result = "error ipfs pin status: init"
		case tvIpfs.PinStatus_WORK:
			// never happen
			log.Debug("PullCidServiceProtocol->HandleRequest: tvIpfs.IpfsGetObject Work")
			info.resp.Status = ReqStatus_ERR
			info.resp.Result = "error ipfs pin status: work"
		default:
			// never happen
			log.Error("PullCidServiceProtocol->HandleRequest: tvIpfs.IpfsGetObject default")
			info.resp.Status = ReqStatus_ERR
			info.resp.Result = fmt.Errorf("error ipfs pin status: %v", pinStatus).Error()
		}
	}

	execTask()

	log.Debugf("PullCidServiceProtocol->HandleRequest: pullCidResponse: %v", info)
	log.Debugf("PullCidServiceProtocol->HandleRequest end")
	return nil
}

func (p *PullCidServiceProtocol) HandleResponse(request *pb.CustomProtocolReq, response *pb.CustomProtocolRes) error {
	log.Debugf("PullCidServiceProtocol->HandleResponse begin:\nrequest: %v \nresponse: %v", request, response)
	pullCidRequest := &PullCidRequest{}
	err := p.CustomStreamServiceProtocol.HandleRequest(request, pullCidRequest)
	if err != nil {
		return err
	}
	log.Debugf("PullCidServiceProtocol->HandleRequest: pullCidRequest: %v", pullCidRequest)

	p.commicateInfoListMutex.RLock()
	commicateInfo := p.commicateInfoList[pullCidRequest.CID]
	p.commicateInfoListMutex.RUnlock()
	if commicateInfo == nil {
		log.Debugf("PullCidServiceProtocol->HandleResponse: commicateInfo is nil")
		return fmt.Errorf("PullCidServiceProtocol->HandleResponse: commicateInfo is nil")
	}

	err = p.CustomStreamServiceProtocol.HandleResponse(response, commicateInfo.resp)
	if err != nil {
		return err
	}
	log.Debugf("PullCidServiceProtocol->HandleResponse: pullCidResponse: %v", commicateInfo.resp)

	// exec ipfs pin operation is finished
	if commicateInfo.resp.Status == ReqStatus_FINISH || commicateInfo.resp.Status == ReqStatus_ERR || commicateInfo.resp.Status == ReqStatus_TIMEOUT {
		commicateInfo.concurrentReqMutex.Lock()
		for i := 0; i < commicateInfo.concurrentReqCount; i++ {
			select {
			case commicateInfo.done <- commicateInfo.resp:
			default:
				log.Debugf("PullCidServiceProtocol->HandleResponse: commicateInfo.done is full")
			}
		}
		commicateInfo.concurrentReqMutex.Unlock()

		p.commicateInfoListMutex.Lock()
		delete(p.commicateInfoList, pullCidRequest.CID)
		p.commicateInfoListMutex.Unlock()

		switch commicateInfo.resp.PinStatus {
		case tvIpfs.PinStatus_PINNED, tvIpfs.PinStatus_ALREADY_PINNED:
			err = p.uploadContentToProvider(commicateInfo.resp.CID, pullCidRequest.StorageProviderList)
			if err != nil {
				return err
			}
			err = p.saveCidInfoToDkvs(commicateInfo.resp.CID)
			if err != nil {
				return err
			}
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
