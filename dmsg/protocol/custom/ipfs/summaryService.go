package syncfile

import (
	"fmt"
	"time"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	ipfspb "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/ipfs/pb"
	storageprovider "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/ipfs/storageProvider"
	"google.golang.org/protobuf/proto"
)

var (
	NftApiKeyList = []string{
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkaWQ6ZXRocjoweDgxOTgwNzg4Y2UxQjY3MDQyM2Y1NzAyMDQ2OWM0MzI3YzNBNzU5YzciLCJpc3MiOiJuZnQtc3RvcmFnZSIsImlhdCI6MTY5MDUyODU1MjcxMywibmFtZSI6InRlc3QxIn0.vslsn8tAWUtZ0BZjcxhyMrcuufwfZ7fTMpF_DrojF4c",
	}

	Web3ApiKeyList = []string{
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkaWQ6ZXRocjoweDAyYzZEYkJBMTQyOTA1MzliZjgwNkEzRkNDRDgzMDFmNWNjNTQ2ZDIiLCJpc3MiOiJ3ZWIzLXN0b3JhZ2UiLCJpYXQiOjE2OTA2ODkxMjg3NDUsIm5hbWUiOiJ0ZXN0In0.nhArwLJYjFwTiW1-SSRPyrCCczyYQ4T2PAHcShFZXqg",
	}
)

type SummaryServiceProtocol struct {
	customProtocol.CustomStreamServiceProtocol
	uploadManager *storageprovider.UploadManager
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
	p.uploadManager = storageprovider.NewUploaderManager()
	p.uploadManager.AddNftUploader(NftApiKeyList[0])
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

	err = p.upload3rdIpfsProvider(summaryReq.CID)
	if err != nil {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROVIDER,
			Result: "SummaryServiceProtocol->HandleRequest: upload3rdIpfsProvider error: " + err.Error(),
		}
		logger.Debugf(retCode.Result)
		return responseContent, retCode, nil
	}

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

func (p *SummaryServiceProtocol) AddNftUploader(apikey string) {
	p.uploadManager.AddNftUploader(apikey)
}

func (p *SummaryServiceProtocol) AddWeb3Uploader(apikey string) {
	p.uploadManager.AddWeb3Uploader(apikey)
}

func (p *SummaryServiceProtocol) upload3rdIpfsProvider(cid string) error {
	// sh := tvbaseIpfs.GetIpfsShellProxy()
	if len(p.uploadManager.NftUploaderList) == 0 {
		return fmt.Errorf("SummaryServiceProtocol->upload3rdIpfsProvider: no ntfUploder service")
	}
	nftUploader := p.uploadManager.NftUploaderList[0]
	uploadTimeout := 30 * time.Minute
	nftUploader.Upload(cid, uploadTimeout)
	return nil
}
