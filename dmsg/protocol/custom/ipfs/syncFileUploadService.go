package syncfile

import (
	tvbaseIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	ipfspb "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/ipfs/pb"
	"google.golang.org/protobuf/proto"
)

type SyncFileUploadService struct {
	customProtocol.CustomStreamServiceProtocol
}

func NewSyncFileUploadService() (*SyncFileUploadService, error) {
	p := &SyncFileUploadService{}
	err := p.Init()
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *SyncFileUploadService) Init() error {
	p.CustomStreamServiceProtocol.Init(PID_SERVICE_SYNCFILE_UPLOAD)
	return nil
}

func (p *SyncFileUploadService) HandleRequest(request *pb.CustomProtocolReq) (
	responseContent []byte, retCode *pb.RetCode, err error) {
	logger.Debugf("SyncFileUploadService->HandleRequest begin:\nrequest.BasicData: %v", request.BasicData)

	syncFileReq := &ipfspb.SyncFileReq{}
	err = proto.Unmarshal(request.Content, syncFileReq)

	if err != nil {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROTOCOL,
			Result: "proto.Unmarshal error:" + err.Error(),
		}
		logger.Debugf(retCode.Result)
		return responseContent, retCode, nil
	}
	logger.Debugf("SyncFileUploadService->HandleRequest: syncFileReq.CID: %v", syncFileReq.CID)

	syncFileRes := &ipfspb.SyncFileRes{
		CID: syncFileReq.CID,
	}
	responseContent, _ = proto.Marshal(syncFileRes)

	sh := tvbaseIpfs.GetIpfsShellProxy()
	isPin := sh.IsPin(syncFileReq.CID)
	if isPin {
		retCode = &pb.RetCode{
			Code:   CODE_ALREADY_PIN,
			Result: "already pin",
		}
		return responseContent, retCode, nil
	}

	if syncFileReq.Data == nil || len(syncFileReq.Data) == 0 {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_IPFS,
			Result: "syncFileReq.Data is empty",
		}
		logger.Errorf(retCode.Result)
		return responseContent, retCode, nil
	}

	cid, err := sh.BlockPutVo(syncFileReq.Data)
	if err != nil {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_IPFS,
			Result: "sh.BlockPutVo error: " + err.Error(),
		}
		logger.Errorf(retCode.Result)
		return responseContent, retCode, nil
	}

	if cid != syncFileReq.CID {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROTOCOL,
			Result: "calculated CID is different from the input parameter CID, calculated cid:" + cid,
		}
		logger.Errorf(retCode.Result)
		return responseContent, retCode, nil
	}

	err = sh.DirectPin(cid, p.Ctx)
	if err != nil {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_IPFS,
			Result: "sh.DirectPin error:" + err.Error(),
		}
		logger.Errorf(retCode.Result)
		return responseContent, retCode, nil
	}

	retCode = &pb.RetCode{
		Code:   CODE_SUCC,
		Result: "success",
	}

	logger.Debugf("SyncFileUploadService->HandleRequest end")
	return responseContent, retCode, nil
}
