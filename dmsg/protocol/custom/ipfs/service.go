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

	// "github.com/orcaman/concurrent-map"
	"github.com/libp2p/go-libp2p/core/crypto"
	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	tvIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	"github.com/tinyverse-web3/tvbase/dkvs"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	ipfspb "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/ipfs/pb"
)

// service

const StorageKeyPrefix = "/storage012345678901234567890123456789/ipfs012345678901234567890123456789/"

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

type ipfsUpload func(providerName string, cid string) error

type uploadTask struct {
	cid string
}

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

type serviceCommicateInfo struct {
	resp *ipfspb.IpfsRes
}

type IpfsServiceProtocol struct {
	PriKey crypto.PrivKey
	customProtocol.CustomStreamServiceProtocol
	commicateInfoList      map[string]*serviceCommicateInfo
	commicateInfoListMutex sync.RWMutex
	tvBaseService          tvbaseCommon.TvBaseService
	ipfsProviderList       map[string]*ipfsProviderList
	storageInfoList        *map[string]any
}

var pullCidServiceProtocol *IpfsServiceProtocol

func GetServiceProtocol(tvBaseService tvbaseCommon.TvBaseService) (*IpfsServiceProtocol, error) {
	if pullCidServiceProtocol == nil {
		pullCidServiceProtocol = &IpfsServiceProtocol{}
		err := pullCidServiceProtocol.Init(tvBaseService)
		if err != nil {
			return nil, err
		}
	}
	return pullCidServiceProtocol, nil
}

func (p *IpfsServiceProtocol) Init(tvBaseService tvbaseCommon.TvBaseService) error {
	err := tvIpfs.CheckIpfsCmd()
	if err != nil {
		return err
	}
	if p.PriKey == nil {
		prikey, err := dkvs.GetPriKeyBySeed(pid)
		if err != nil {
			log.Errorf("IpfsServiceProtocol->Init: GetPriKeyBySeed err: %v", err)
			return err
		}
		p.PriKey = prikey

	}
	p.CustomStreamServiceProtocol.Init(pid, customProtocol.DataType_PROTO3)
	p.tvBaseService = tvBaseService
	p.storageInfoList = &map[string]any{}
	p.ipfsProviderList = make(map[string]*ipfsProviderList)
	p.initIpfsProviderTask(NftProvider, NftApiKey, NftPostURL, UploadInterval, UploadTimeout, p.httpUploadCidContent)
	p.initIpfsProviderTask(Web3Provider, Web3ApiKey, Web3PostURL, UploadInterval, UploadTimeout, p.httpUploadCidContent)
	return nil
}

func (p *IpfsServiceProtocol) HandleRequest(request *pb.CustomProtocolReq) error {
	log.Debugf("IpfsServiceProtocol->HandleRequest begin:\nrequest: %v", request)
	ipfsReq := &ipfspb.IpfsReq{}
	err := p.CustomStreamServiceProtocol.HandleRequest(request, ipfsReq)
	if err != nil {
		return err
	}
	log.Debugf("IpfsServiceProtocol->HandleRequest: ipfsReq: %v", ipfsReq)

	// log.Debugf("IpfsServiceProtocol->HandleRequest: pullCidResponse: %v", info)
	log.Debugf("IpfsServiceProtocol->HandleRequest end")
	return nil
}

func (p *IpfsServiceProtocol) HandleResponse(request *pb.CustomProtocolReq, response *pb.CustomProtocolRes) error {
	log.Debugf("IpfsServiceProtocol->HandleResponse begin:\nrequest: %v \nresponse: %v", request, response)
	ipfsReq := &ipfspb.IpfsReq{}
	err := p.CustomStreamServiceProtocol.HandleRequest(request, ipfsReq)
	if err != nil {
		return err
	}
	log.Debugf("IpfsServiceProtocol->HandleRequest: ipfsReq: %v", ipfsReq)

	p.commicateInfoListMutex.RLock()
	commicateInfo := p.commicateInfoList[ipfsReq.CID]
	p.commicateInfoListMutex.RUnlock()
	if commicateInfo == nil {
		log.Debugf("IpfsServiceProtocol->HandleResponse: commicateInfo is nil")
		return fmt.Errorf("IpfsServiceProtocol->HandleResponse: commicateInfo is nil")
	}

	err = p.CustomStreamServiceProtocol.HandleResponse(response, commicateInfo.resp)
	if err != nil {
		return err
	}
	log.Debugf("IpfsServiceProtocol->HandleResponse: pullCidResponse: %v", commicateInfo.resp)

	// exec ipfs pin operation is finished

	log.Debugf("IpfsServiceProtocol->HandleResponse end")
	return nil
}

func (p *IpfsServiceProtocol) uploadContentToProvider(cid string, storageProviderList []string) error {
	log.Debugf("IpfsServiceProtocol->uploadContentToProvider begin: \ncid:%s\nstorageProviderList:%v",
		cid, storageProviderList)
	for _, storageProvider := range storageProviderList {
		(*p.storageInfoList)[storageProvider] = storageProvider
		p.asyncUploadCidContent(storageProvider, cid)
	}
	log.Debugf("IpfsServiceProtocol->uploadContentToProvider end")
	return nil
}

func (p *IpfsServiceProtocol) saveCidInfoToDkvs(cid string) error {
	log.Debugf("IpfsServiceProtocol->saveCidInfoToDkvs begin: cid:%s", cid)
	dkvsKey := StorageKeyPrefix + cid
	pubkeyData, err := crypto.MarshalPublicKey(p.PriKey.GetPublic())
	if err != nil {
		log.Errorf("IpfsServiceProtocol->saveCidInfoToDkvs: crypto.MarshalPublicKey error: %v", err)
		return err
	}
	peerID := p.tvBaseService.GetHost().ID().String()
	isExistKey := p.tvBaseService.GetDkvsService().Has(dkvsKey)
	if isExistKey {
		value, _, _, _, _, err := p.tvBaseService.GetDkvsService().Get(dkvsKey)
		if err != nil {
			log.Errorf("IpfsServiceProtocol->saveCidInfoToDkvs: GetDkvsService->Get error: %v", err)
			return err
		}
		err = json.Unmarshal(value, p.storageInfoList)
		if err != nil {
			log.Warnf("IpfsServiceProtocol->saveCidInfoToDkvs: json.Unmarshal old dkvs value error: %v", err)
			return nil
		}
		if (*p.storageInfoList)[peerID] != nil {
			log.Debugf("IpfsServiceProtocol->saveCidInfoToDkvs: peerID is already exist in dkvs, peerID: %s", peerID)
			return nil
		}
	}

	(*p.storageInfoList)[peerID] = peerID
	value, err := json.Marshal(p.storageInfoList)
	if err != nil {
		log.Errorf("IpfsServiceProtocol->saveCidInfoToDkvs: json marshal new dkvs value error: %v", err)
		return err
	}
	issuetime := dkvs.TimeNow()
	ttl := dkvs.GetTtlFromDuration(time.Hour * 24 * 30 * 12 * 100) // about 100 year
	sig, err := p.PriKey.Sign(dkvs.GetRecordSignData(dkvsKey, value, pubkeyData, issuetime, ttl))
	if err != nil {
		log.Errorf("IpfsServiceProtocol->saveCidInfoToDkvs: SignDataByEcdsa: %v", err)
		return err
	}

	err = p.tvBaseService.GetDkvsService().Put(dkvsKey, value, pubkeyData, issuetime, ttl, sig)
	if err != nil {
		log.Errorf("IpfsServiceProtocol->saveCidInfoToDkvs: Put error: %v", err)
		return err
	}
	log.Debugf("IpfsServiceProtocol->saveCidInfoToDkvs end")
	return nil
}

func (p *IpfsServiceProtocol) httpUploadCidContent(providerName string, cid string) error {
	// the same API key exceeds 30 request within 10 seconds, the rate limit will be triggered
	// https://nft.storage/api-docs/  https://web3.storage/docs/reference/http-api/
	log.Debugf("IpfsServiceProtocol->httpUploadCidContent begin: providerName:%s, cid: %s", providerName, cid)
	provider := p.ipfsProviderList[providerName]
	if provider == nil {
		log.Errorf("IpfsServiceProtocol->httpUploadCidContent: provider is nil, providerName: %s", providerName)
		return fmt.Errorf("IpfsServiceProtocol->httpUploadCidContent: provider is nil, providerName: %s", providerName)
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
		log.Errorf("IpfsServiceProtocol->httpUploadCidContent: file too large(<100MB), bufSize:%v", bufSize)
		return fmt.Errorf("IpfsServiceProtocol->httpUploadCidContent: file too large(<100MB), bufSize:%v", bufSize)
	}

	client := &http.Client{
		Timeout: provider.timeout,
	}
	req, err := http.NewRequest("POST", provider.uploadUrl, buf)
	if err != nil {
		log.Errorf("IpfsServiceProtocol->httpUploadCidContent: http.NewRequest error: %v", err)
		return nil
	}

	req.Header.Set("Authorization", "Bearer "+provider.apiKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("IpfsServiceProtocol->httpUploadCidContent: client.Do error: %v", err)
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("IpfsServiceProtocol->httpUploadCidContent: ioutil.ReadAll error: %v", err)
		return err
	}

	log.Debugf("IpfsServiceProtocol->httpUploadCidContent: body: %s", string(responseBody))
	log.Debugf("IpfsServiceProtocol->httpUploadCidContent end")
	return nil
}

func (p *IpfsServiceProtocol) initIpfsProviderTask(providerName string, apiKey string, uploadUrl string, interval time.Duration, timeout time.Duration, uploadFunc ipfsUpload) error {
	if p.ipfsProviderList[providerName] != nil {
		log.Errorf("IpfsServiceProtocol->initIpfsProviderTask: ipfsProviderTaskList[providerName] is not nil")
		return fmt.Errorf("IpfsServiceProtocol->initIpfsProviderTask: ipfsProviderTaskList[providerName] is not nil")
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

func (p *IpfsServiceProtocol) asyncUploadCidContent(providerName string, cid string) error {
	log.Debugf("IpfsServiceProtocol->asyncUploadCidContent begin:\nproviderName:%s\n cid: %v",
		providerName, cid)
	dkvsKey := StorageKeyPrefix + cid
	isExistKey := p.tvBaseService.GetDkvsService().Has(dkvsKey)
	if isExistKey {
		value, _, _, _, _, err := p.tvBaseService.GetDkvsService().Get(dkvsKey)
		if err != nil {
			log.Errorf("IpfsServiceProtocol->asyncUploadCidContent: GetDkvsService->Get error: %v", err)
			return err
		}
		err = json.Unmarshal(value, p.storageInfoList)
		if err != nil {
			log.Warnf("IpfsServiceProtocol->asyncUploadCidContent: json.Unmarshal old dkvs value error: %v", err)
			return err
		}
		if (*p.storageInfoList)[providerName] != nil {
			log.Debugf("IpfsServiceProtocol->asyncUploadCidContent: provider is already exist in dkvs, providerName: %s", providerName)
			return nil
		}
	}

	ipfsProviderTask := p.ipfsProviderList[providerName]
	if ipfsProviderTask == nil {
		log.Errorf("IpfsServiceProtocol->asyncUploadCidContent: ipfsProviderList[providerName] is nil")
		return fmt.Errorf("IpfsServiceProtocol->asyncUploadCidContent: ipfsProviderList[providerName] is nil")
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
			log.Debugf("IpfsServiceProtocol->asyncUploadCidContent: ipfsProviderTask.timer.C")
		case <-done:
			ipfsProviderTask.timer.Stop()
			ipfsProviderTask.isRun = false
			log.Debugf("IpfsServiceProtocol->asyncUploadCidContent end: done")
			return nil
		case <-p.Ctx.Done():
			ipfsProviderTask.timer.Stop()
			ipfsProviderTask.isRun = false
			log.Debugf("IpfsServiceProtocol->asyncUploadCidContent end: p.Ctx.Done()")
			return p.Ctx.Err()
		}
	}
}
