package pullcid

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	tvbaseCommon "github.com/tinyverse-web3/tvbase/common"
	tvIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	"github.com/tinyverse-web3/tvbase/dkvs"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	"github.com/tinyverse-web3/tvbase/tvbase"
	tvutilCrypto "github.com/tinyverse-web3/tvutil/crypto"
	keyUtil "github.com/tinyverse-web3/tvutil/key"
)

const pullCidPID = "pullcid"

const StorageKeyPrefix = "/tvnode/storage/"

var NftApiKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkaWQ6ZXRocjoweDgxOTgwNzg4Y2UxQjY3MDQyM2Y1NzAyMDQ2OWM0MzI3YzNBNzU5YzciLCJpc3MiOiJuZnQtc3RvcmFnZSIsImlhdCI6MTY5MDUyODU1MjcxMywibmFtZSI6InRlc3QxIn0.vslsn8tAWUtZ0BZjcxhyMrcuufwfZ7fTMpF_DrojF4c"
var Web3StorageApiKey = ""

type clientCommicateInfo struct {
	data            any
	responseSignal  chan any
	createTimestamp int64
}

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
	commicateInfoList map[string]*clientCommicateInfo
}

var pullCidClientProtocol *PullCidClientProtocol
var pullCidServiceProtocol *PullCidServiceProtocol

func GetPullCidClientProtocol(tvbase *tvbase.TvBase) (*PullCidClientProtocol, error) {
	if pullCidClientProtocol == nil {
		pullCidClientProtocol = &PullCidClientProtocol{}
		err := pullCidClientProtocol.Init()
		if err != nil {
			return nil, err
		}
	}
	return pullCidClientProtocol, nil
}

var PullCidServicePriKey *ecdsa.PrivateKey

func (p *PullCidClientProtocol) Init() error {
	if PullCidServicePriKey == nil {
		var err error
		PullCidServicePriKey, _, err = keyUtil.GenerateEcdsaKey(pullCidPID)
		if err != nil {
			customProtocol.Logger.Errorf("PullCidClientProtocol->Init: GenerateEcdsaKey err: %v", err)
			return err
		}
	}

	p.CustomStreamClientProtocol.Init(pullCidPID)
	p.commicateInfoList = make(map[string]*clientCommicateInfo)
	return nil
}

func (p *PullCidClientProtocol) HandleResponse(request *pb.CustomProtocolReq, response *pb.CustomProtocolRes) error {
	pullCidResponse := &PullCidResponse{
		Status: tvIpfs.PinStatus_UNKNOW,
	}
	err := p.CustomStreamClientProtocol.HandleResponse(response, pullCidResponse)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidClientProtocol->HandleResponse: err: %v", err)
		return err
	}
	requestInfo := p.commicateInfoList[pullCidResponse.CID]
	if requestInfo == nil {
		customProtocol.Logger.Errorf("PullCidClientProtocol->HandleResponse: requestInfo is nil, cid: %s", pullCidResponse.CID)
		return fmt.Errorf("PullCidClientProtocol->HandleResponse: requestInfo is nil, cid: %s", pullCidResponse.CID)
	}

	requestInfo.responseSignal <- pullCidResponse
	delete(p.commicateInfoList, pullCidResponse.CID)

	return nil
}

func (p *PullCidClientProtocol) Request(peerId string, request *PullCidRequest, options ...any) (*PullCidResponse, error) {
	_, err := cid.Decode(request.CID)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidClientProtocol->Request: cid.Decode: err: %v, cid: %s", err, request.CID)
		return nil, err
	}

	var timeout time.Duration = 3 * time.Second
	if len(options) > 0 {
		var ok bool
		timeout, ok = options[0].(time.Duration)
		if !ok {
			customProtocol.Logger.Errorf("PullCidClientProtocol->Request: timeout is not time.Duration")
			return nil, fmt.Errorf("PullCidClientProtocol->Request: timeout is not time.Duration")
		}
	}

	if len(p.commicateInfoList) > 0 {
		go p.cleanCommicateInfoList(30 * time.Second)
	}

	requestInfo := &clientCommicateInfo{
		data:            request,
		createTimestamp: time.Now().Unix(),
		responseSignal:  make(chan any),
	}
	p.commicateInfoList[request.CID] = requestInfo

	err = p.CustomStreamClientProtocol.Request(peerId, request)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidClientProtocol->Request: err: %v", err)
		return nil, err
	}

	if timeout <= 0 {
		customProtocol.Logger.Warnf("PullCidClientProtocol->Request: timeout <= 0")
		return nil, nil
	}

	select {
	case responseObject := <-requestInfo.responseSignal:
		pullCidResponse, ok := responseObject.(*PullCidResponse)
		if !ok {
			customProtocol.Logger.Errorf("PullCidClientProtocol->Request: responseData is not PullCidResponse")
			return nil, err
		}
		return pullCidResponse, nil
	case <-time.After(timeout):
		delete(p.commicateInfoList, request.CID)
		return nil, err
	case <-p.Ctx.Done():
		delete(p.commicateInfoList, request.CID)
		return nil, p.Ctx.Err()
	}
}

func (p *PullCidClientProtocol) cleanCommicateInfoList(expiration time.Duration) {
	for id, v := range p.commicateInfoList {
		if time.Since(time.Unix(v.createTimestamp, 0)) > expiration {
			delete(p.commicateInfoList, id)
		}
	}
	customProtocol.Logger.Debug("PullCidClientProtocol->cleanCommicateInfoList: clean commicateInfoList")
}

// service
type PullCidServiceProtocol struct {
	customProtocol.CustomStreamServiceProtocol
	commicateInfoList      map[string]*serviceCommicateInfo
	commicateInfoListMutex sync.Mutex
	tvBaseService          tvbaseCommon.TvBaseService
}

func GetPullCidServiceProtocol(tvBaseService tvbaseCommon.TvBaseService) *PullCidServiceProtocol {
	if pullCidServiceProtocol == nil {
		pullCidServiceProtocol = &PullCidServiceProtocol{}
		pullCidServiceProtocol.Init(tvBaseService)
	}
	return pullCidServiceProtocol
}

func (p *PullCidServiceProtocol) Init(tvBaseService tvbaseCommon.TvBaseService) {
	p.CustomStreamServiceProtocol.Init(pullCidPID)
	p.tvBaseService = tvBaseService
	p.commicateInfoList = make(map[string]*serviceCommicateInfo)
}

func (p *PullCidServiceProtocol) HandleRequest(request *pb.CustomProtocolReq) error {
	pullCidRequest := &PullCidRequest{}
	err := p.CustomStreamServiceProtocol.HandleRequest(request, pullCidRequest)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidServiceProtocol->HandleRequest: err: %v", err)
		return err
	}

	err = tvIpfs.CheckIpfsCmd()
	if err != nil {
		customProtocol.Logger.Errorf("PullCidServiceProtocol->HandleRequest: err: %v", err)
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
			customProtocol.Logger.Errorf("PullCidServiceProtocol->HandleRequest: err: %v", err)
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
		customProtocol.Logger.Debugf("PullCidServiceProtocol->HandleRequest: cid: %v, pullCidResponse: %v",
			pullCidRequest.CID, pullCidResponse)
	}()

	<-timer.C
	return nil
}

func (p *PullCidServiceProtocol) HandleResponse(request *pb.CustomProtocolReq, response *pb.CustomProtocolRes) error {
	pullCidRequest := &PullCidRequest{}
	err := p.CustomStreamServiceProtocol.HandleRequest(request, pullCidRequest)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidClientProtocol->HandleResponse: err: %v", err)
		return err
	}

	commicateInfo := p.commicateInfoList[pullCidRequest.CID]
	if commicateInfo == nil {
		customProtocol.Logger.Warnf("PullCidClientProtocol->HandleResponse: commicateInfo is nil, cid: %s", pullCidRequest.CID)
		return fmt.Errorf("PullCidClientProtocol->HandleResponse: commicateInfo is nil, cid: %s", pullCidRequest.CID)
	}

	pullCidResponse, ok := commicateInfo.data.(*PullCidResponse)
	if !ok {
		customProtocol.Logger.Infof("PullCidClientProtocol->HandleResponse: pullCidResponse is nil, cid: %s", pullCidResponse.CID)
		return fmt.Errorf("PullCidClientProtocol->HandleResponse: pullCidResponse is nil, cid: %s", pullCidRequest.CID)
	}

	err = p.CustomStreamServiceProtocol.HandleResponse(response, pullCidResponse)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidClientProtocol->HandleResponse: err: %v", err)
		return err
	}

	switch pullCidResponse.Status {
	case tvIpfs.PinStatus_ERR, tvIpfs.PinStatus_PINNED, tvIpfs.PinStatus_TIMEOUT:
		delete(p.commicateInfoList, pullCidRequest.CID)
	default:
		customProtocol.Logger.Debugf("PullCidClientProtocol->HandleResponse: cid: %v, pullCidResponse: %v, status: %v, pullcid working....",
			pullCidRequest.CID, pullCidResponse, pullCidResponse.Status)
	}

	if pullCidResponse.Status == tvIpfs.PinStatus_PINNED {
		var storageInfoList *map[string]any = &map[string]any{}
		err = p.uploadContentToProvider(pullCidResponse.CID, pullCidRequest.StorageProviderList, storageInfoList)
		if err != nil {
			return err
		}
		err = p.saveCidInfoToDkvs(pullCidResponse.CID, storageInfoList)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *PullCidServiceProtocol) uploadContentToProvider(cid string, storageProviderList []string, storageInfoList *map[string]any) error {
	customProtocol.Logger.Debugf("PullCidServiceProtocol->uploadContentToProvider begin: \ncid:%s\nstorageProviderList:%v",
		cid, storageProviderList)
	if len(storageProviderList) > 0 {
		content, elapsedTime, err := tvIpfs.IpfsBlockGet(cid, p.Ctx)
		if err != nil {
			customProtocol.Logger.Errorf("PullCidServiceProtocol->uploadContentToProvider: err: %v", err)
			return err
		}
		customProtocol.Logger.Debugf("PullCidServiceProtocol->uploadContentToProvider: ipfs block get:\ncontent:%v \nelapsedTime: %v", content, elapsedTime)
	}
	for _, storageProvider := range storageProviderList {
		customProtocol.Logger.Debugf("PullCidServiceProtocol->uploadContentToProvider: storageProvider:%s", storageProvider)
		switch storageProvider {
		case "nft":
			(*storageInfoList)["nft"] = storageProvider
			go p.uploadContentToNft(p.Ctx, cid)
		case "web3storage":
			(*storageInfoList)["web3storage"] = storageProvider
		}
	}
	customProtocol.Logger.Debugf("PullCidServiceProtocol->uploadContentToProvider end")
	return nil
}

func (p *PullCidServiceProtocol) saveCidInfoToDkvs(cid string, storageInfoList *map[string]any) error {
	customProtocol.Logger.Debugf("PullCidServiceProtocol->saveCidInfoToDkvs begin: cid:%s", cid)
	dkvsKey := StorageKeyPrefix + cid
	pubkeyData, err := keyUtil.ECDSAPublicKeyToProtoBuf(&PullCidServicePriKey.PublicKey)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidServiceProtocol->saveCidInfoToDkvs: ECDSAPublicKeyToProtoBuf error %v", err)
		return err
	}
	value, _, _, _, _, err := p.tvBaseService.GetDkvsService().Get(dkvsKey)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidServiceProtocol->saveCidInfoToDkvs: GetDkvsService->Get error: %v", err)
		return err
	}
	err = json.Unmarshal(value, storageInfoList)
	if err != nil {
		customProtocol.Logger.Warnf("PullCidServiceProtocol->saveCidInfoToDkvs: json.Unmarshal old dkvs value error: %v", err)
	}
	peerID := p.tvBaseService.GetHost().ID().String()
	(*storageInfoList)[peerID] = peerID
	value, err = json.Marshal(storageInfoList)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidServiceProtocol->saveCidInfoToDkvs: json marshal new dkvs value error: %v", err)
		return err
	}
	sig, err := tvutilCrypto.SignDataByEcdsa(PullCidServicePriKey, value)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidServiceProtocol->saveCidInfoToDkvs: SignDataByEcdsa: %v", err)
		return err
	}
	maxTTL := dkvs.GetTtlFromDuration(time.Hour * 24 * 30 * 12 * 100) // about 100 year
	err = p.tvBaseService.GetDkvsService().Put(dkvsKey, []byte(value), pubkeyData, dkvs.TimeNow(), maxTTL, sig)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidServiceProtocol->saveCidInfoToDkvs: Put error: %v", err)
		return err
	}
	customProtocol.Logger.Debugf("PullCidServiceProtocol->saveCidInfoToDkvs end")
	return nil
}

func (p *PullCidServiceProtocol) uploadContentToNft(ctx context.Context, cid string) error {
	// NFT.Storage 接受每次上传大小高达31GiB的存储请求! 每次上传可以包含单个文件或文件目录。
	// 如果您使用的是 HTTP API，则需要对超过 100MB 的文件进行一些手动拆分。有关详细信息，请参阅HTTP API https://nft.storage/api-docs/
	// 如果 API 使用以下方式收到超过 30 个请求，则会触发速率限制：在 10 秒窗口内使用相同的 API 密钥。
	customProtocol.Logger.Debugf("PullCidServiceProtocol->uploadContentToNft begin: cid: %v", cid)
	content, _, err := tvIpfs.IpfsBlockGet(cid, ctx)
	if err != nil {
		return err
	}

	requestBody := &bytes.Buffer{}
	writer := multipart.NewWriter(requestBody)
	fileWriter, err := writer.CreateFormFile("file", cid)
	if err != nil {
		customProtocol.Logger.Debugf("PullCidServiceProtocol->uploadContentToNft: CreateFormFile error: %v", err)
		return nil
	}

	fileBuffer := bytes.NewBuffer(content)
	_, err = fileWriter.Write(fileBuffer.Bytes())
	if err != nil {
		customProtocol.Logger.Debugf("PullCidServiceProtocol->uploadContentToNft: fileWriter.Write error: %v", err)
		return nil
	}
	writer.WriteField("Content-Type", writer.FormDataContentType())
	err = writer.Close()
	if err != nil {
		customProtocol.Logger.Debugf("PullCidServiceProtocol->uploadContentToNft: writer.Close error: %v", err)
		return nil
	}

	requestBodySize := len(requestBody.Bytes())
	if requestBodySize >= 100*1024*1024 {
		// TODO 100MB need split small files ,links to car, implement it
		customProtocol.Logger.Errorf("PullCidServiceProtocol->uploadContentToNft: file too large, requestBodySize:%v", requestBodySize)
		return fmt.Errorf("uploadContentToNft->uploadContentToNft: file too large, requestBodySize:%v", requestBodySize)
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://api.nft.storage/upload", requestBody)
	if err != nil {
		customProtocol.Logger.Errorf("TestNftStorageUpload: http.NewRequest error: %v", err)
		return nil
	}

	req.Header.Set("Authorization", "Bearer "+NftApiKey)

	resp, err := client.Do(req)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidServiceProtocol->uploadContentToNft: client.Do error: %v", err)
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidServiceProtocol->uploadContentToNft: ioutil.ReadAll error: %v", err)
		return err
	}

	customProtocol.Logger.Debugf("PullCidServiceProtocol->uploadContentToNft: body: %s", string(responseBody))
	customProtocol.Logger.Debugf("PullCidServiceProtocol->uploadContentToNft end")
	return nil
}

func (p *PullCidServiceProtocol) asyncMethod() {
	taskQueue := make(chan int)

	// 启动一个goroutine来处理任务队列
	go func() {
		for task := range taskQueue {
			// 执行任务的操作
			fmt.Printf("执行任务：%d\n", task)
		}
	}()

	// 添加任务到任务队列
	timer := time.NewTicker(3 * time.Second)
	for {
		<-timer.C // 定时器到期后，从定时器的通道中读取数据
		taskQueue <- 1
	}

	// 关闭任务队列
	close(taskQueue)
}
