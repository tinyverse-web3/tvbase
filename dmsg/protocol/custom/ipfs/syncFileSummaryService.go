package syncfile

import (
	"context"
	"fmt"
	"time"

	tvbaseIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
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

	DefaultUploadTimeout = 30 * time.Minute
)

type SyncFileSummaryService struct {
	customProtocol.CustomStreamServiceProtocol
	uploadManager *storageprovider.UploadManager
}

func NewSyncFileSummaryService() (*SyncFileSummaryService, error) {
	p := &SyncFileSummaryService{}
	err := p.Init()
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *SyncFileSummaryService) Init() error {
	p.CustomStreamServiceProtocol.Init(PID_SERVICE_SYNCFILE_SUMMARY)
	p.uploadManager = storageprovider.NewUploaderManager()
	p.uploadManager.AddNftUploader(NftApiKeyList[0])
	return nil
}

func (p *SyncFileSummaryService) HandleRequest(request *pb.CustomProtocolReq) (
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
	logger.Debugf("SummaryServiceProtocol->HandleRequest: CID: %v", summaryReq.CID)

	sh := tvbaseIpfs.GetIpfsShellProxy()
	isPin := sh.IsPin(summaryReq.CID)
	if !isPin {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_NOPIN,
			Result: "local ipfs no pin data",
		}
		return responseContent, retCode, nil
	}
	summaryRes, err := p.upload3rdIpfsProvider(summaryReq)
	if err != nil {
		retCode = &pb.RetCode{
			Code:   CODE_ERROR_PROVIDER,
			Result: err.Error(),
		}
		logger.Debugf(retCode.Result)
		return responseContent, retCode, nil
	}

	responseContent, _ = proto.Marshal(summaryRes)
	retCode = &pb.RetCode{
		Code:   CODE_SUCC,
		Result: fmt.Sprintf("%+v", summaryRes),
	}
	logger.Debugf("SummaryServiceProtocol->HandleRequest end")
	return responseContent, retCode, nil
}

func (p *SyncFileSummaryService) AddNftUploader(apikey string) {
	p.uploadManager.AddNftUploader(apikey)
}

func (p *SyncFileSummaryService) AddWeb3Uploader(apikey string) {
	p.uploadManager.AddWeb3Uploader(apikey)
}

func (p *SyncFileSummaryService) upload3rdIpfsProvider(summaryReq *ipfspb.SummaryReq) (summaryRes *ipfspb.SummaryRes, err error) {
	summaryRes = &ipfspb.SummaryRes{
		CID:          summaryReq.CID,
		ProivderList: make([]*ipfspb.Provider, 0),
	}
	providerStoreCid, err := p.uploadNftProvider(summaryReq.CID)
	if err != nil {
		return summaryRes, err
	}

	if summaryReq.MaxProviderCount == 0 {
		summaryReq.MaxProviderCount = 20
	}
	peerIdList, err := p.queryPeerList(summaryReq.CID, int(summaryReq.MaxProviderCount))
	if err != nil {
		return summaryRes, err
	}
	summaryRes.ProivderList = append(summaryRes.ProivderList, &ipfspb.Provider{
		CID:        summaryReq.CID,
		PeerIdList: peerIdList,
	})

	if providerStoreCid != summaryReq.CID {
		peerIdList, err := p.queryPeerList(providerStoreCid, int(summaryReq.MaxProviderCount))
		if err != nil {
			return summaryRes, err
		}
		summaryRes.ProivderList = append(summaryRes.ProivderList, &ipfspb.Provider{
			CID:        providerStoreCid,
			PeerIdList: peerIdList,
		})
	}
	return summaryRes, nil
}

func (p *SyncFileSummaryService) queryPeerList(cid string, maxProviderCount int) ([]string, error) {
	timeout := time.Duration(maxProviderCount) * 10 * time.Second
	ctx, cancel := context.WithTimeout(p.Ctx, timeout)
	defer cancel()

	_, peerIdList, err := tvbaseIpfs.RoutingFindProvs(ctx, cid, maxProviderCount)
	if err != nil {
		return nil, err
	}
	return peerIdList, nil
}

func (p *SyncFileSummaryService) uploadNftProvider(cid string) (providerStoreCid string, err error) {
	nftUploader := p.uploadManager.NftUploaderList[0]

	isOk, resp, err := nftUploader.CheckCid(cid)
	if err != nil {
		return providerStoreCid, fmt.Errorf("check cid error:%v", err)
	}
	if isOk {
		logger.Debugf("upload cid:%v is already pin, resp: %v", cid, resp)
		return cid, nil
	}

	isOk, resp, err = nftUploader.Upload(cid, DefaultUploadTimeout)
	if err != nil {
		return providerStoreCid, fmt.Errorf("upload error:%v", err)
	}
	if !isOk {
		return providerStoreCid, fmt.Errorf("upload response isn't ok, response:%v", resp)
	}

	value, ok := resp["value"].(map[string]interface{})
	if !ok {
		return providerStoreCid, fmt.Errorf("failure to convert value json object")
	}

	providerStoreCid, ok = value["cid"].(string)
	if !ok {
		return providerStoreCid, fmt.Errorf("failure to convert cid json object")
	}

	return providerStoreCid, nil
}
