package syncfile

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	tvbaseIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	ipfspb "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/ipfs/pb"
	"google.golang.org/protobuf/proto"
)

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

type SummaryServiceProtocol struct {
	customProtocol.CustomStreamServiceProtocol
	pullTaskList     map[string]string
	ipfsProviderList map[string]*ipfsProviderList
}

var summaryServiceProtocol *SummaryServiceProtocol

func GetSummaryServiceProtocol() (*SummaryServiceProtocol, error) {
	if summaryServiceProtocol == nil {
		summaryServiceProtocol = &SummaryServiceProtocol{}
		err := summaryServiceProtocol.Init()
		if err != nil {
			return nil, err
		}
	}
	return summaryServiceProtocol, nil
}

func (p *SummaryServiceProtocol) Init() error {
	p.CustomStreamServiceProtocol.Init(CID_STATUS_SERVICE)
	p.ipfsProviderList = make(map[string]*ipfsProviderList)
	p.pullTaskList = make(map[string]string)
	p.initIpfsProviderTask(NftProvider, NftApiKey, NftPostURL, UploadInterval, UploadTimeout, p.httpUploadCidContent)
	p.initIpfsProviderTask(Web3Provider, Web3ApiKey, Web3PostURL, UploadInterval, UploadTimeout, p.httpUploadCidContent)
	return nil
}

func (p *SummaryServiceProtocol) HandleRequest(request *pb.CustomProtocolReq) (
	responseContent []byte, retCode *pb.RetCode, err error) {
	logger.Debugf("SummaryServiceProtocol->HandleRequest begin:\nrequest.BasicData: %v", request.BasicData)

	summaryReq := &ipfspb.SummaryReq{}
	err = proto.Unmarshal(request.Content, summaryReq)
	if err != nil {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROTOCOL,
			Result: "SummaryServiceProtocol->HandleRequest: proto.Unmarshal error: " + err.Error(),
		}
		logger.Debugf(retCode.Result)
		return responseContent, retCode, nil
	}
	logger.Debugf("SummaryServiceProtocol->HandleRequest: syncFileReq.CID: %v", summaryReq.CID)

	// sh := tvbaseIpfs.GetIpfsShellProxy()

	summaryRes := &ipfspb.SummaryRes{
		CID: summaryReq.CID,
	}
	responseContent, _ = proto.Marshal(summaryRes)
	retCode = &pb.RetCode{
		Code:   0,
		Result: "success",
	}
	logger.Debugf("SummaryServiceProtocol->HandleRequest end")
	return responseContent, retCode, nil
}

func (p *SummaryServiceProtocol) uploadContentToProvider(cid string, storageProviderList []string) error {
	logger.Debugf("IpfsServiceProtocol->uploadContentToProvider begin: \ncid:%s\nstorageProviderList:%v",
		cid, storageProviderList)
	for _, storageProvider := range storageProviderList {
		p.asyncUploadCidContent(storageProvider, cid)
	}
	logger.Debugf("IpfsServiceProtocol->uploadContentToProvider end")
	return nil
}

func (p *SummaryServiceProtocol) httpUploadCidContent(providerName string, cid string) error {
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

func (p *SummaryServiceProtocol) initIpfsProviderTask(
	providerName string,
	apiKey string,
	uploadUrl string,
	interval time.Duration,
	timeout time.Duration,
	uploadFunc ipfsUpload) error {
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

func (p *SummaryServiceProtocol) asyncUploadCidContent(providerName string, cid string) error {
	logger.Debugf("IpfsServiceProtocol->asyncUploadCidContent begin:\nproviderName:%s\n cid: %v",
		providerName, cid)

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
