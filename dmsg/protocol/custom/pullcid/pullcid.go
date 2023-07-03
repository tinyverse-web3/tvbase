package pullcid

import (
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/tinyverse-web3/tvbase/common/ipfs"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
)

const pullCidPID = "pullcid"

type lightCommicateInfo struct {
	data            any
	responseSignal  chan any
	createTimestamp int64
}

type serviceCommicateInfo struct {
	data any
}

type PullCidRequest struct {
	CID          string
	CheckTimeout time.Duration
}

type PullCidResponse struct {
	CID            string
	CidContentSize int64
	ElapsedTime    time.Duration
	Status         ipfs.PidStatus
}

// light
type PullCidLightProtocol struct {
	customProtocol.CustomStreamLightProtocol
	commicateInfoList map[string]*lightCommicateInfo
}

var pullCidLightProtocol *PullCidLightProtocol
var pullCidServiceProtocol *PullCidServiceProtocol

func GetPullCidLightProtocol() *PullCidLightProtocol {
	if pullCidLightProtocol == nil {
		pullCidLightProtocol = &PullCidLightProtocol{}
		pullCidLightProtocol.Init()
	}
	return pullCidLightProtocol
}

func (p *PullCidLightProtocol) Init() {
	p.CustomStreamLightProtocol.Init(pullCidPID)
	p.commicateInfoList = make(map[string]*lightCommicateInfo)
}

func (p *PullCidLightProtocol) HandleResponse(request *pb.CustomProtocolReq, response *pb.CustomProtocolRes) error {
	pullCidResponse := &PullCidResponse{}
	err := p.CustomStreamLightProtocol.HandleResponse(response, pullCidResponse)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidLightProtocol->HandleResponse: err: %v", err)
		return err
	}
	requestInfo := p.commicateInfoList[pullCidResponse.CID]
	if requestInfo == nil {
		customProtocol.Logger.Errorf("PullCidLightProtocol->HandleResponse: requestInfo is nil, cid: %s", pullCidResponse.CID)
		return fmt.Errorf("PullCidLightProtocol->HandleResponse: requestInfo is nil, cid: %s", pullCidResponse.CID)
	}

	requestInfo.responseSignal <- pullCidResponse
	delete(p.commicateInfoList, pullCidResponse.CID)

	return nil
}

func (p *PullCidLightProtocol) Request(request *PullCidRequest, options ...any) (*PullCidResponse, error) {
	_, err := cid.Decode(request.CID)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidLightProtocol->Request: cid.Decode: err: %v, cid: %s", err, request.CID)
		return nil, err
	}

	var timeout time.Duration = 3 * time.Second
	if len(options) > 0 {
		var ok bool
		timeout, ok = options[0].(time.Duration)
		if !ok {
			customProtocol.Logger.Errorf("PullCidLightProtocol->Request: timeout is not time.Duration")
			return nil, fmt.Errorf("PullCidLightProtocol->Request: timeout is not time.Duration")
		}
	}

	if len(p.commicateInfoList) > 0 {
		go p.cleanCommicateInfoList(30 * time.Second)
	}

	requestInfo := &lightCommicateInfo{
		data:            request,
		createTimestamp: time.Now().Unix(),
		responseSignal:  make(chan any),
	}
	p.commicateInfoList[request.CID] = requestInfo

	err = p.CustomStreamLightProtocol.Request(request)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidLightProtocol->Request: err: %v", err)
		return nil, err
	}

	if timeout <= 0 {
		customProtocol.Logger.Warnf("PullCidLightProtocol->Request: timeout <= 0")
		return nil, nil
	}

	select {
	case responseObject := <-requestInfo.responseSignal:
		pullCidResponse, ok := responseObject.(*PullCidResponse)
		if !ok {
			customProtocol.Logger.Errorf("PullCidLightProtocol->Request: responseData is not PullCidResponse")
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

func (p *PullCidLightProtocol) cleanCommicateInfoList(expiration time.Duration) {
	for id, v := range p.commicateInfoList {
		if time.Since(time.Unix(v.createTimestamp, 0)) > expiration {
			delete(p.commicateInfoList, id)
		}
	}
	customProtocol.Logger.Debug("PullCidLightProtocol->cleanCommicateInfoList: clean commicateInfoList")
}

// service
type PullCidServiceProtocol struct {
	customProtocol.CustomStreamServiceProtocol
	commicateInfoList map[string]*serviceCommicateInfo
}

func GetPullCidServiceProtocol() *PullCidServiceProtocol {
	if pullCidServiceProtocol == nil {
		pullCidServiceProtocol = &PullCidServiceProtocol{}
		pullCidServiceProtocol.Init()
	}
	return pullCidServiceProtocol
}

func (p *PullCidServiceProtocol) Init() {
	p.CustomStreamServiceProtocol.Init(pullCidPID)
	p.commicateInfoList = make(map[string]*serviceCommicateInfo)
}

func (p *PullCidServiceProtocol) HandleRequest(request *pb.CustomProtocolReq) error {
	pullCidRequest := &PullCidRequest{}
	err := p.CustomStreamServiceProtocol.HandleRequest(request, pullCidRequest)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidServiceProtocol->HandleRequest: err: %v", err)
		return err
	}

	err = ipfs.CheckIpfsCmd()
	if err != nil {
		customProtocol.Logger.Errorf("PullCidServiceProtocol->HandleRequest: err: %v", err)
		return err
	}

	if p.commicateInfoList[pullCidRequest.CID] != nil {
		return nil
	}

	pullCidResponse := &PullCidResponse{
		CID: pullCidRequest.CID,
	}
	p.commicateInfoList[pullCidRequest.CID] = &serviceCommicateInfo{
		data: pullCidResponse,
	}

	timeout := pullCidRequest.CheckTimeout

	if timeout <= 0 {
		// default timeout is 3 minute
		timeout = 3 * time.Minute
	} else if timeout < 10*time.Second {
		// min timeout is 10 second
		timeout = 10 * time.Second
	} else if timeout > 3*time.Hour {
		// max timeout is 3 hour
		timeout = 3 * time.Hour
	}

	go func() {
		CidContentSize, elapsedTime, pinStatus, err := ipfs.IpfsGetObject(pullCidRequest.CID, p.Ctx, timeout)
		if err != nil {
			customProtocol.Logger.Errorf("PullCidServiceProtocol->HandleRequest: err: %v", err)
			return
		}

		pullCidResponse.CidContentSize = CidContentSize
		pullCidResponse.ElapsedTime = elapsedTime
		pullCidResponse.Status = pinStatus
		customProtocol.Logger.Debugf("PullCidServiceProtocol->HandleRequest: pullCidResponse: %v", pullCidResponse)
	}()

	return nil
}

func (p *PullCidServiceProtocol) HandleResponse(request *pb.CustomProtocolReq, response *pb.CustomProtocolRes) error {
	pullCidRequest := &PullCidRequest{}
	err := p.CustomStreamServiceProtocol.HandleRequest(request, pullCidRequest)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidLightProtocol->HandleResponse: err: %v", err)
		return err
	}

	commicateInfo := p.commicateInfoList[pullCidRequest.CID]
	if commicateInfo == nil {
		customProtocol.Logger.Warnf("PullCidLightProtocol->HandleResponse: commicateInfo is nil, cid: %s", pullCidRequest.CID)
		return fmt.Errorf("PullCidLightProtocol->HandleResponse: commicateInfo is nil, cid: %s", pullCidRequest.CID)
	}

	pullCidResponse, ok := commicateInfo.data.(*PullCidResponse)
	if !ok {
		customProtocol.Logger.Infof("PullCidLightProtocol->HandleResponse: pullCidResponse is nil, cid: %s", pullCidResponse.CID)
		return fmt.Errorf("PullCidLightProtocol->HandleResponse: pullCidResponse is nil, cid: %s", pullCidRequest.CID)
	}

	err = p.CustomStreamServiceProtocol.HandleResponse(response, pullCidResponse)
	if err != nil {
		customProtocol.Logger.Errorf("PullCidLightProtocol->HandleResponse: err: %v", err)
		return err
	}

	switch pullCidResponse.Status {
	case ipfs.PinStatus_ERR, ipfs.PinStatus_PINNED, ipfs.PinStatus_TIMEOUT:
		delete(p.commicateInfoList, pullCidRequest.CID)
	}

	return nil
}
