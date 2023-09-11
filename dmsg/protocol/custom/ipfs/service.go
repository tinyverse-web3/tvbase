package syncfile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	tvbaseIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	ipfspb "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/ipfs/pb"
	"google.golang.org/protobuf/proto"
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

type FileSyncServiceProtocol struct {
	customProtocol.CustomStreamServiceProtocol
	tvBaseService    tvbaseCommon.TvBaseService
	ipfsProviderList map[string]*ipfsProviderList
	storageInfoList  *map[string]any
}

var pullCidServiceProtocol *FileSyncServiceProtocol

func GetServiceProtocol(tvBaseService tvbaseCommon.TvBaseService) (*FileSyncServiceProtocol, error) {
	if pullCidServiceProtocol == nil {
		pullCidServiceProtocol = &FileSyncServiceProtocol{}
		err := pullCidServiceProtocol.Init(tvBaseService)
		if err != nil {
			return nil, err
		}
	}
	return pullCidServiceProtocol, nil
}

func (p *FileSyncServiceProtocol) Init(tvBaseService tvbaseCommon.TvBaseService) error {
	p.CustomStreamServiceProtocol.Init(SYNCIPFSFILEPID)
	p.tvBaseService = tvBaseService
	p.storageInfoList = &map[string]any{}
	p.ipfsProviderList = make(map[string]*ipfsProviderList)
	p.initIpfsProviderTask(NftProvider, NftApiKey, NftPostURL, UploadInterval, UploadTimeout, p.httpUploadCidContent)
	p.initIpfsProviderTask(Web3Provider, Web3ApiKey, Web3PostURL, UploadInterval, UploadTimeout, p.httpUploadCidContent)
	return nil
}

func (p *FileSyncServiceProtocol) HandleRequest(request *pb.CustomProtocolReq) (
	responseContent []byte, retCode *pb.RetCode, err error) {
	logger.Debugf("FileSyncServiceProtocol->HandleRequest begin:\nrequest.BasicData: %v", request.BasicData)

	syncFileReq := &ipfspb.SyncFileReq{}
	syncFileRes := &ipfspb.SyncFileRes{
		CID: syncFileReq.CID,
	}
	responseContent, _ = proto.Marshal(syncFileRes)

	err = proto.Unmarshal(request.Content, syncFileReq)
	if err != nil {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROTOCOL,
			Result: "FileSyncServiceProtocol->HandleRequest: proto.Unmarshal error: " + err.Error(),
		}
		logger.Debugf(retCode.Result)
		return responseContent, retCode, nil
	}
	logger.Debugf("FileSyncServiceProtocol->HandleRequest: syncFileReq.CID: %v", syncFileReq.CID)

	sh := tvbaseIpfs.GetIpfsShellProxy()
	isPin := sh.IsPin(syncFileReq.CID)
	if !isPin {
		cid, err := sh.BlockPutVo(syncFileReq.Data)
		if err != nil {
			retCode = &pb.RetCode{
				Code:   CODE_ERROR_IPFS,
				Result: "FileSyncServiceProtocol->HandleRequest: sh.BlockPutVo error: " + err.Error(),
			}
			logger.Errorf(retCode.Result)
		}
		if cid != syncFileReq.CID {
			retCode = &pb.RetCode{
				Code:   CODE_ERROR_PROTOCOL,
				Result: "FileSyncServiceProtocol->HandleRequest: calculated CID is different from the input parameter CID, calculated cid :" + cid,
			}
			logger.Errorf(retCode.Result)
		} else {
			err = sh.DirectPin(cid, p.Ctx)
			if err != nil {
				logger.Errorf("FileSyncServiceProtocol->HandleRequest: sh.Unpin error: %v, cid: %s", err, cid)
			}
			if err != nil {
				retCode = &pb.RetCode{
					Code:   CODE_ERROR_IPFS,
					Result: "FileSyncServiceProtocol->HandleRequest: sh.DirectPin error: " + err.Error(),
				}
			} else {
				retCode = &pb.RetCode{
					Code:   CODE_PIN,
					Result: "success",
				}
			}
			if syncFileReq.Accelerate {
				stat, err := sh.ObjectStat(cid)
				if err != nil {
					logger.Errorf("FileSyncServiceProtocol->HandleRequest: sh.ObjectStat error: %v", err)
					retCode = &pb.RetCode{
						Code:   CODE_PIN,
						Result: "success, but accelerate fail",
					}
				} else {
					// TODO : experiment, need to optimization
					size := stat.CumulativeSize + stat.BlockSize
					const MinSize = 1024 * 10
					const CommonSize = 1024 * 1024
					const MinTimeout = 30 * time.Second
					const CommonTimeout = 60 * time.Second
					const MaxTimeout = 5 * 60 * time.Second
					timeout := MinTimeout
					if size < MinSize {
						timeout = MinTimeout
					} else if size < CommonSize {
						timeout = CommonTimeout
					} else {
						timeout = MaxTimeout
					}
					timeoutCtx, cancel := context.WithTimeout(p.Ctx, timeout)
					go func() {
						err = sh.RecursivePin(cid, timeoutCtx)
						if err != nil {
							logger.Errorf("FileSyncServiceProtocol->HandleRequest: sh.RecursivePin error: %v, cid: %s", err, cid)
						}
						cancel()
					}()
				}
			}
		}
	} else {
		retCode = &pb.RetCode{
			Code:   CODE_ALREADY_PIN,
			Result: "already pin",
		}
	}
	logger.Debugf("FileSyncServiceProtocol->HandleRequest end")
	return responseContent, retCode, nil
}

func (p *FileSyncServiceProtocol) uploadContentToProvider(cid string, storageProviderList []string) error {
	logger.Debugf("IpfsServiceProtocol->uploadContentToProvider begin: \ncid:%s\nstorageProviderList:%v",
		cid, storageProviderList)
	for _, storageProvider := range storageProviderList {
		(*p.storageInfoList)[storageProvider] = storageProvider
		p.asyncUploadCidContent(storageProvider, cid)
	}
	logger.Debugf("IpfsServiceProtocol->uploadContentToProvider end")
	return nil
}

func (p *FileSyncServiceProtocol) httpUploadCidContent(providerName string, cid string) error {
	// the same API key exceeds 30 request within 10 seconds, the rate limit will be triggered
	// https://nft.storage/api-docs/  https://web3.storage/docs/reference/http-api/
	logger.Debugf("IpfsServiceProtocol->httpUploadCidContent begin: providerName:%s, cid: %s", providerName, cid)
	provider := p.ipfsProviderList[providerName]
	if provider == nil {
		logger.Errorf("IpfsServiceProtocol->httpUploadCidContent: provider is nil, providerName: %s", providerName)
		return fmt.Errorf("IpfsServiceProtocol->httpUploadCidContent: provider is nil, providerName: %s", providerName)
	}

	timeoutCtx, cancel := context.WithTimeout(p.Ctx, 10*time.Second)
	defer cancel()
	content, _, err := tvbaseIpfs.IpfsBlockGet(cid, timeoutCtx)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(content)
	bufSize := len(buf.Bytes())
	if bufSize >= 100*1024*1024 {
		// TODO over 100MB need to be split using CAR,, implement it
		logger.Errorf("IpfsServiceProtocol->httpUploadCidContent: file too large(<100MB), bufSize:%v", bufSize)
		return fmt.Errorf("IpfsServiceProtocol->httpUploadCidContent: file too large(<100MB), bufSize:%v", bufSize)
	}

	client := &http.Client{
		Timeout: provider.timeout,
	}
	req, err := http.NewRequest("POST", provider.uploadUrl, buf)
	if err != nil {
		logger.Errorf("IpfsServiceProtocol->httpUploadCidContent: http.NewRequest error: %v", err)
		return nil
	}

	req.Header.Set("Authorization", "Bearer "+provider.apiKey)

	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("IpfsServiceProtocol->httpUploadCidContent: client.Do error: %v", err)
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("IpfsServiceProtocol->httpUploadCidContent: ioutil.ReadAll error: %v", err)
		return err
	}

	logger.Debugf("IpfsServiceProtocol->httpUploadCidContent: body: %s", string(responseBody))
	logger.Debugf("IpfsServiceProtocol->httpUploadCidContent end")
	return nil
}

func (p *FileSyncServiceProtocol) initIpfsProviderTask(providerName string, apiKey string, uploadUrl string, interval time.Duration, timeout time.Duration, uploadFunc ipfsUpload) error {
	if p.ipfsProviderList[providerName] != nil {
		logger.Errorf("IpfsServiceProtocol->initIpfsProviderTask: ipfsProviderTaskList[providerName] is not nil")
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

func (p *FileSyncServiceProtocol) asyncUploadCidContent(providerName string, cid string) error {
	logger.Debugf("IpfsServiceProtocol->asyncUploadCidContent begin:\nproviderName:%s\n cid: %v",
		providerName, cid)
	dkvsKey := StorageKeyPrefix + cid
	isExistKey := p.tvBaseService.GetDkvsService().Has(dkvsKey)
	if isExistKey {
		value, _, _, _, _, err := p.tvBaseService.GetDkvsService().Get(dkvsKey)
		if err != nil {
			logger.Errorf("IpfsServiceProtocol->asyncUploadCidContent: GetDkvsService->Get error: %v", err)
			return err
		}
		err = json.Unmarshal(value, p.storageInfoList)
		if err != nil {
			logger.Warnf("IpfsServiceProtocol->asyncUploadCidContent: json.Unmarshal old dkvs value error: %v", err)
			return err
		}
		if (*p.storageInfoList)[providerName] != nil {
			logger.Debugf("IpfsServiceProtocol->asyncUploadCidContent: provider is already exist in dkvs, providerName: %s", providerName)
			return nil
		}
	}

	ipfsProviderTask := p.ipfsProviderList[providerName]
	if ipfsProviderTask == nil {
		logger.Errorf("IpfsServiceProtocol->asyncUploadCidContent: ipfsProviderList[providerName] is nil")
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
			logger.Debugf("IpfsServiceProtocol->asyncUploadCidContent: ipfsProviderTask.timer.C")
		case <-done:
			ipfsProviderTask.timer.Stop()
			ipfsProviderTask.isRun = false
			logger.Debugf("IpfsServiceProtocol->asyncUploadCidContent end: done")
			return nil
		case <-p.Ctx.Done():
			ipfsProviderTask.timer.Stop()
			ipfsProviderTask.isRun = false
			logger.Debugf("IpfsServiceProtocol->asyncUploadCidContent end: p.Ctx.Done()")
			return p.Ctx.Err()
		}
	}
}
