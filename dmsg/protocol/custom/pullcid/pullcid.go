package pullcid

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/crypto"
	tvIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	"github.com/tinyverse-web3/tvbase/dkvs"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
)

const PID_SERVICE_PULLCID = "pullcid"
const loggerName = "dmsg.protocol.custom.pullid"

var logger = ipfsLog.Logger(loggerName)

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

type PullCidServiceProtocol struct {
	PriKey crypto.PrivKey
	customProtocol.CustomStreamServiceProtocol
	commicateInfoList sync.Map
	mutex             sync.Mutex
}

func NewPullCidService() (*PullCidServiceProtocol, error) {
	pullCidService := &PullCidServiceProtocol{}
	err := pullCidService.Init()
	if err != nil {
		return nil, err
	}
	return pullCidService, nil
}

func (p *PullCidServiceProtocol) Init() error {
	if p.PriKey == nil {
		prikey, err := dkvs.GetPriKeyBySeed(PID_SERVICE_PULLCID)
		if err != nil {
			logger.Errorf("PullCidServiceProtocol->Init: GetPriKeyBySeed err: %v", err)
			return err
		}
		p.PriKey = prikey
	}
	p.CustomStreamServiceProtocol.Init(PID_SERVICE_PULLCID)
	return nil
}

func (p *PullCidServiceProtocol) HandleRequest(request *pb.CustomProtocolReq) (
	responseContent []byte, retCode *pb.RetCode, err error) {
	logger.Debugf("PullCidServiceProtocol->HandleRequest begin:\nrequest: %v", request)
	pullCidRequest := &PullCidRequest{}

	const CODE_ERROR_PROTOCOL = -1
	const CODE_SUCC = 0

	if request.PID != p.PID {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROTOCOL,
			Result: "CustomStreamServiceProtocol->HandleRequest: protocol cid is not equal , request.PID: " + request.PID + ", p.PID: " + p.PID,
		}
		logger.Error(retCode.Result)
		return responseContent, retCode, nil
	}

	err = json.Unmarshal(request.Content, pullCidRequest)
	if err != nil {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROTOCOL,
			Result: "CustomStreamServiceProtocol->HandleRequest: json unmarshal error:" + err.Error(),
		}
		logger.Error(retCode.Result)
		return responseContent, retCode, nil
	}

	err = tvIpfs.CheckIpfsCmd()
	if err != nil {
		return nil, nil, err
	}

	p.mutex.Lock()
	_, ok := p.commicateInfoList.Load(pullCidRequest.CID)
	if !ok {
		p.mutex.Unlock()
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROTOCOL,
			Result: "CustomStreamServiceProtocol->HandleRequest: it working, p.PID: " + p.PID,
		}
		logger.Error(retCode.Result)
		return responseContent, retCode, nil
	}
	p.commicateInfoList.Store(pullCidRequest.CID, &PullCidResponse{})
	p.mutex.Unlock()

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

	cidContentSize, elapsedTime, pinStatus, err := tvIpfs.IpfsGetObject(pullCidRequest.CID, p.Ctx, maxCheckTime)
	pullCidResponse := &PullCidResponse{
		CID:            pullCidRequest.CID,
		CidContentSize: cidContentSize,
		ElapsedTime:    elapsedTime,
		Status:         pinStatus,
	}
	if err != nil {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROTOCOL,
			Result: fmt.Sprintf("CustomStreamServiceProtocol->HandleRequest: it pin error: %+v, resp: %+v", err, pullCidResponse),
		}
		logger.Error(retCode.Result)
		return responseContent, retCode, nil
	}

	responseContent, err = json.Marshal(pullCidResponse)
	if err != nil {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROTOCOL,
			Result: "CustomStreamServiceProtocol->HandleRequest: json marshal error:" + err.Error(),
		}
		logger.Error(retCode.Result)
		return responseContent, retCode, nil
	}
	switch pullCidResponse.Status {
	case tvIpfs.PinStatus_PINNED:
		retCode = &pb.RetCode{
			Code:   0,
			Result: fmt.Sprintf("CustomStreamServiceProtocol->HandleRequest: it's already pinned, resp: %+v", pullCidResponse),
		}
		logger.Debugf(retCode.Result)
	case tvIpfs.PinStatus_ERR:
		retCode = &pb.RetCode{
			Code:   0,
			Result: fmt.Sprintf("CustomStreamServiceProtocol->HandleRequest: it pin failed, resp: %+v", pullCidResponse),
		}
		logger.Debugf(retCode.Result)
	case tvIpfs.PinStatus_TIMEOUT:
		retCode = &pb.RetCode{
			Code:   0,
			Result: fmt.Sprintf("CustomStreamServiceProtocol->HandleRequest: it pin timeout, resp: %+v", pullCidResponse),
		}
		logger.Debugf(retCode.Result)
	default:
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROTOCOL,
			Result: fmt.Sprintf("CustomStreamServiceProtocol->HandleRequest: it working, resp: %+v", pullCidResponse),
		}
		logger.Debugf("PullCidServiceProtocol->HandleResponse: cid: %v, pullCidResponse: %v, status: %v, pullcid working....",
			pullCidRequest.CID, pullCidResponse, pullCidResponse.Status)
	}

	p.commicateInfoList.Delete(pullCidRequest.CID)
	logger.Debugf("PullCidServiceProtocol->HandleRequest end\ncid: %v\npullCidResponse: %v", pullCidRequest.CID, pullCidResponse)
	return responseContent, retCode, nil
}
