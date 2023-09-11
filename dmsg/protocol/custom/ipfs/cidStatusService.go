package syncfile

import (
	tvbaseIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	ipfspb "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/ipfs/pb"
	"google.golang.org/protobuf/proto"
)

type CidStatusServiceProtocol struct {
	customProtocol.CustomStreamServiceProtocol
}

var cidStatusServiceProtocol *CidStatusServiceProtocol

func GetCidStatusServiceProtocol() (*CidStatusServiceProtocol, error) {
	if cidStatusServiceProtocol == nil {
		cidStatusServiceProtocol = &CidStatusServiceProtocol{}
		err := cidStatusServiceProtocol.Init()
		if err != nil {
			return nil, err
		}
	}
	return cidStatusServiceProtocol, nil
}

func (p *CidStatusServiceProtocol) Init() error {
	p.CustomStreamServiceProtocol.Init(CID_STATUS_SERVICE)
	return nil
}

func (p *CidStatusServiceProtocol) HandleRequest(request *pb.CustomProtocolReq) (
	responseContent []byte, retCode *pb.RetCode, err error) {
	logger.Debugf("CidStatusServiceProtocol->HandleRequest begin:\nrequest.BasicData: %v", request.BasicData)

	checkIpfsStatusReq := &ipfspb.CidStatusReq{}
	err = proto.Unmarshal(request.Content, checkIpfsStatusReq)
	if err != nil {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROTOCOL,
			Result: "CidStatusServiceProtocol->HandleRequest: proto.Unmarshal error: " + err.Error(),
		}
		logger.Debugf(retCode.Result)
		return responseContent, retCode, nil
	}
	logger.Debugf("CidStatusServiceProtocol->HandleRequest: syncFileReq.CID: %v", checkIpfsStatusReq.CID)

	sh := tvbaseIpfs.GetIpfsShellProxy()

	checkIpfsStatusRes := &ipfspb.CidStatusRes{
		CID:   checkIpfsStatusReq.CID,
		IsPin: sh.IsPin(checkIpfsStatusReq.CID),
	}
	responseContent, _ = proto.Marshal(checkIpfsStatusRes)
	retCode = &pb.RetCode{
		Code:   0,
		Result: "success",
	}
	logger.Debugf("CidStatusServiceProtocol->HandleRequest end")
	return responseContent, retCode, nil
}
