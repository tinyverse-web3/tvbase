package pullcid

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
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

const StorageKeyPrefix = "/tvnode/storage/ipfs/"

type clientCommicateInfo struct {
	data            any
	responseSignal  chan any
	createTimestamp int64
}

type serviceCommicateInfo struct {
	data any
}

type PullCidRequest struct {
	CID          string
	MaxCheckTime time.Duration
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
		var peerIDList map[string]any = make(map[string]any)
		// TODO 提供选项，可选择让用户上传CID对应数据至NTF.STORAGE和WEB3.STORAGE，比如付费数据或重要数据

		// dkvs save cid info
		key := StorageKeyPrefix + pullCidResponse.CID
		pubkey := &PullCidServicePriKey.PublicKey

		pubkeyData, err := keyUtil.ECDSAPublicKeyToProtoBuf(pubkey)
		if err != nil {
			customProtocol.Logger.Errorf("PullCidClientProtocol->HandleResponse: ECDSAPublicKeyToProtoBuf error %v", err)
			return err
		}

		value, _, _, _, _, err := p.tvBaseService.GetDkvsService().Get(key)
		if err != nil {
			customProtocol.Logger.Errorf("PullCidClientProtocol->HandleResponse: GetDkvsService->Get error: %v", err)
			return err
		}

		err = json.Unmarshal(value, &peerIDList)
		if err != nil {
			customProtocol.Logger.Warnf("PullCidClientProtocol->HandleResponse: json.Unmarshal old dkvs value error: %v", err)
		}

		peerID := p.tvBaseService.GetHost().ID().String()
		peerIDList[peerID] = peerID
		value, err = json.Marshal(peerIDList)
		if err != nil {
			customProtocol.Logger.Errorf("PullCidClientProtocol->HandleResponse: json marshal new dkvs value error: %v", err)
			return err
		}

		sig, err := tvutilCrypto.SignDataByEcdsa(PullCidServicePriKey, value)
		if err != nil {
			customProtocol.Logger.Errorf("PullCidClientProtocol->HandleResponse: SignDataByEcdsa: %v", err)
			return err
		}
		maxTTL := dkvs.GetTtlFromDuration(time.Hour * 24 * 30 * 12 * 100) // about 100 year
		p.tvBaseService.GetDkvsService().Put(key, []byte(value), pubkeyData, dkvs.TimeNow(), maxTTL, sig)
	}

	return nil
}
