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

type CustomStreamProtocolResponseParam struct {
	PID     string
	Service customProtocol.CustomStreamProtocolService
}

type CustomStreamProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
	pid      string
}

func NewCustomStreamProtocolAdapter() *CustomStreamProtocolAdapter {
	ret := &CustomStreamProtocolAdapter{}
	return ret
}

func (adapter *CustomStreamProtocolAdapter) init() {
	adapter.protocol.Request = &pb.CustomProtocolReq{}
	adapter.protocol.Response = &pb.CustomProtocolRes{}
}

func (adapter *CustomStreamProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CUSTOM_STREAM_RES
}

func (adapter *CustomStreamProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CUSTOM_STREAM_REQ
}

func (adapter *CustomStreamProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolRes + "/" + adapter.pid)
}

func (adapter *CustomStreamProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolReq + "/" + adapter.pid)
}

func (adapter *CustomStreamProtocolAdapter) SetProtocolResponseFailRet(errMsg string) {
	request, ok := adapter.protocol.Response.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter->SetProtocolResponseFailRet: data is not CustomProtocolReq")
		return
	}
	request.RetCode = dmsgProtocol.NewFailRetCode(errMsg)
}

func (adapter *CustomStreamProtocolAdapter) SetProtocolResponseRet(code int32, result string) {
	request, ok := adapter.protocol.Response.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomStreamProtocolAdapter->SetProtocolResponseRet: protoData is not CustomProtocolRes")
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *CustomStreamProtocolAdapter) GetRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.Request.(*pb.CustomProtocolReq)
	if !ok {
		tvLog.Logger.Errorf("CustomStreamProtocolAdapter->GetRequestBasicData: protoData is not CustomProtocolReq")
		return nil
	}
	return request.BasicData
}

func (adapter *CustomStreamProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	request, ok := adapter.protocol.Response.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomStreamProtocolAdapter->GetResponseBasicData: protoData is not CustomProtocolReq")
		return nil
	}
	return request.BasicData
}

func (adapter *CustomStreamProtocolAdapter) InitResponse(basicData *pb.BasicData, dataList ...any) error {
	if len(dataList) < 1 {
		return errors.New("CustomStreamProtocolAdapter:InitResponse: dataList need contain customStreamProtocolResponseParam")
	}

	customStreamProtocolResponseParam, ok := dataList[0].(*CustomStreamProtocolResponseParam)
	if !ok {
		tvLog.Logger.Errorf("CustomStreamProtocolAdapter->InitResponse: fail to cast dataList[0] CustomStreamProtocolResponseParam)")
		return nil
	}

	request, ok := adapter.protocol.Request.(*pb.CustomProtocolReq)
	if !ok {
		tvLog.Logger.Errorf("dmsgService->OnCustomPubsubProtocolResponse: cannot convert to *pb.CustomContentReq")
		return fmt.Errorf("dmsgService->OnCustomPubsubProtocolResponse: cannot convert to *pb.CustomContentReq")
	}

	response := &pb.CustomProtocolRes{
		BasicData: basicData,
		PID:       customStreamProtocolResponseParam.PID,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	adapter.protocol.Response = response

	// get response.Content
	err := customStreamProtocolResponseParam.Service.HandleResponse(request, response)
	if err != nil {
		tvLog.Logger.Errorf("dmsgService->OnCustomStreamProtocolResponse: callback happen err: %v", err)
		return err
	}

	return nil
}

func (adapter *CustomStreamProtocolAdapter) SetResponseSig(sig []byte) error {
	response, ok := adapter.protocol.Response.(*pb.CustomProtocolRes)
	if !ok {
		return errors.New("CustomStreamProtocolAdapter->SetResponseSig: failed to cast request to *pb.ReleaseMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CustomStreamProtocolAdapter) CallRequestCallback() (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnCustomStreamProtocolRequest(adapter.protocol.Request)
	return data, retCode, err
}

func (adapter *CustomStreamProtocolAdapter) CallResponseCallback() (any, error) {
	data, err := adapter.protocol.Callback.OnCustomStreamProtocolResponse(adapter.protocol.Request, adapter.protocol.Response)
	return data, err
}

func NewCustomStreamProtocol(
	ctx context.Context,
	host host.Host,
	pid string,
	protocolService common.ProtocolService,
	protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewCustomStreamProtocolAdapter()
	adapter.pid = pid
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return adapter.protocol
}
