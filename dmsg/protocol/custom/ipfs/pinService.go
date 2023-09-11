package syncfile

import (
	"context"
	"time"

	tvbaseIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	ipfspb "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/ipfs/pb"
	"google.golang.org/protobuf/proto"
)

type PinServiceProtocol struct {
	customProtocol.CustomStreamServiceProtocol
}

var pinServiceProtocol *PinServiceProtocol

func GetPinServiceProtocol() (*PinServiceProtocol, error) {
	if pinServiceProtocol == nil {
		pinServiceProtocol = &PinServiceProtocol{}
		err := pinServiceProtocol.Init()
		if err != nil {
			return nil, err
		}
	}
	return pinServiceProtocol, nil
}

func (p *PinServiceProtocol) Init() error {
	p.CustomStreamServiceProtocol.Init(PIN_SERVICE)
	return nil
}

func (p *PinServiceProtocol) HandleRequest(request *pb.CustomProtocolReq) (
	responseContent []byte, retCode *pb.RetCode, err error) {
	logger.Debugf("PinServiceProtocol->HandleRequest begin:\nrequest.BasicData: %v", request.BasicData)

	pinReq := &ipfspb.PinReq{}
	err = proto.Unmarshal(request.Content, pinReq)
	if err != nil {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROTOCOL,
			Result: "PinServiceProtocol->HandleRequest: proto.Unmarshal error: " + err.Error(),
		}
		logger.Debugf(retCode.Result)
		return responseContent, retCode, nil
	}
	logger.Debugf("PinServiceProtocol->HandleRequest: syncFileReq.CID: %v", pinReq.CID)

	pinRes := &ipfspb.PinRes{
		CID: pinReq.CID,
	}
	responseContent, _ = proto.Marshal(pinRes)
	sh := tvbaseIpfs.GetIpfsShellProxy()
	isPin := sh.IsPin(pinReq.CID)
	if isPin {
		retCode = &pb.RetCode{
			Code:   CODE_ALREADY_PIN,
			Result: "already pin",
		}
		return responseContent, retCode, nil
	}

	// stat, err := sh.ObjectStat(pinReq.CID)
	// if err != nil {
	// 	logger.Errorf("FileSyncServiceProtocol->HandleRequest: sh.ObjectStat error: %v", err)
	// 	retCode = &pb.RetCode{
	// 		Code:   CODE_PIN,
	// 		Result: "success, but accelerate fail",
	// 	}
	// 	return responseContent, retCode, nil
	// }
	// // TODO : experiment, need to optimization
	// size := stat.CumulativeSize + stat.BlockSize
	// const MinSize = 1024 * 10
	// const CommonSize = 1024 * 1024
	const MinTimeout = 30 * time.Second
	// const CommonTimeout = 60 * time.Second
	const MaxTimeout = 5 * 60 * time.Second
	// timeout := MinTimeout
	// if size < MinSize {
	// 	timeout = MinTimeout
	// } else if size < CommonSize {
	// 	timeout = CommonTimeout
	// } else {
	// 	timeout = MaxTimeout
	// }

	timeout := pinReq.Timeout
	if timeout <= 0 {
		timeout = int64(MinTimeout)
	} else if timeout >= int64(MaxTimeout) {
		timeout = int64(MaxTimeout)
	}

	timeoutCtx, cancel := context.WithTimeout(p.Ctx, time.Duration(timeout))
	go func() {
		err = sh.RecursivePin(pinReq.CID, timeoutCtx)
		if err != nil {
			logger.Errorf("FileSyncServiceProtocol->HandleRequest: sh.RecursivePin error: %v, cid: %s", err, pinReq.CID)
		}
		cancel()
	}()

	retCode = &pb.RetCode{
		Code:   CODE_PIN,
		Result: "success",
	}

	logger.Debugf("FileSyncServiceProtocol->HandleRequest end")
	return responseContent, retCode, nil
}
