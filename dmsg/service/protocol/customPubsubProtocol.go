package protocol

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
)

type CustomPubsubProtocolResponseParam struct {
	PID     string
	Service customProtocol.CustomPubsubProtocolService
}

type CustomPubsubProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.PubsubProtocol
	pid      string
}

func NewCustomPubsubProtocolAdapter() *CustomPubsubProtocolAdapter {
	ret := &CustomPubsubProtocolAdapter{}
	return ret
}

func (adapter *CustomPubsubProtocolAdapter) init(customProtocolId string) {
	adapter.pid = customProtocolId
	adapter.protocol.Request = &pb.CustomProtocolReq{}
	adapter.protocol.Response = &pb.CustomProtocolRes{}
}

func (adapter *CustomPubsubProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CUSTOM_STREAM_RES
}

func (adapter *CustomPubsubProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CUSTOM_STREAM_REQ
}

func (adapter *CustomPubsubProtocolAdapter) DestoryProtocol() {
	adapter.protocol.Host.RemoveStreamHandler(protocol.ID(dmsgProtocol.PidCustomProtocolReq + "/" + adapter.pid))
}

func (adapter *CustomPubsubProtocolAdapter) SetProtocolResponseFailRet(errMsg string) {
	request, ok := adapter.protocol.Response.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter->SetProtocolResponseFailRet: data is not CustomProtocolReq")
		return
	}
	request.RetCode = dmsgProtocol.NewFailRetCode(errMsg)
}

func (adapter *CustomPubsubProtocolAdapter) SetProtocolResponseRet(code int32, result string) {
	request, ok := adapter.protocol.Response.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter->SetProtocolResponseRet: data is not CustomProtocolReq")
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *CustomPubsubProtocolAdapter) GetRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.Request.(*pb.CustomProtocolReq)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter GetRequestBasicData data is not CustomProtocolReq")
		return nil
	}
	return request.BasicData
}

func (adapter *CustomPubsubProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	request, ok := adapter.protocol.Response.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomPubsubProtocolAdapter->GetResponseBasicData: data is not CustomProtocolReq")
		return nil
	}
	return request.BasicData
}

func (adapter *CustomPubsubProtocolAdapter) InitResponse(basicData *pb.BasicData, dataList ...any) error {
	if len(dataList) < 1 {
		return errors.New("CustomStreamProtocolAdapter:InitResponse: dataList need contain customPubsubProtocolResponseParam")
	}

	responseParam, ok := dataList[0].(*CustomPubsubProtocolResponseParam)
	if !ok {
		tvLog.Logger.Errorf("CustomPubsubProtocolAdapter->InitResponse: fail to cast dataList[0] customPubsubProtocolResponseParam)")
		return nil
	}

	request, ok := adapter.protocol.Request.(*pb.CustomProtocolReq)
	if !ok {
		tvLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolResponse: cannot convert to *pb.CustomContentReq")
		return fmt.Errorf("dmsgService->OnCustomPubsubProtocolResponse: cannot convert to *pb.CustomContentReq")
	}

	response := &pb.CustomProtocolRes{
		BasicData: basicData,
		PID:       responseParam.PID,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	adapter.protocol.Response = response

	// get response.Content
	err := responseParam.Service.HandleResponse(request, response)
	if err != nil {
		tvLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolResponse: callback happen err: %v", err)
		return err
	}

	return nil
}

func (adapter *CustomPubsubProtocolAdapter) SetResponseSig(sig []byte) error {
	response, ok := adapter.protocol.Response.(*pb.CustomProtocolRes)
	if !ok {
		return errors.New("CustomPubsubProtocolAdapter->SetResponseSig: failed to cast request to *pb.ReleaseMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CustomPubsubProtocolAdapter) CallRequestCallback() (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnCustomPubsubProtocolRequest(adapter.protocol.Request)
	return data, retCode, err
}

func (adapter *CustomPubsubProtocolAdapter) CallResponseCallback() (any, error) {
	data, err := adapter.protocol.Callback.OnCustomPubsubProtocolResponse(adapter.protocol.Request, adapter.protocol.Response)
	return data, err
}

func (adapter *CustomPubsubProtocolAdapter) GetResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.Response.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func NewCustomPubsubProtocol(
	ctx context.Context,
	host host.Host,
	customProtocolId string,
	protocolService common.ProtocolService,
	protocolCallback common.PubsubProtocolCallback) *common.PubsubProtocol {
	adapter := NewCustomPubsubProtocolAdapter()
	protocol := common.NewPubsubProtocol(host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init(customProtocolId)
	return protocol
}
