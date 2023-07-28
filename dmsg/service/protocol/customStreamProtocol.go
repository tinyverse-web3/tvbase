package protocol

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
)

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

func (adapter *CustomStreamProtocolAdapter) DestoryProtocol() {
	adapter.protocol.Host.RemoveStreamHandler(protocol.ID(dmsgProtocol.PidCustomProtocolReq + "/" + adapter.pid))
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
		tvLog.Logger.Errorf("CustomStreamProtocolAdapter InitResponse data is not CustomProtocolReq")
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *CustomStreamProtocolAdapter) GetRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.Request.(*pb.CustomProtocolReq)
	if !ok {
		tvLog.Logger.Errorf("CustomStreamProtocolAdapter InitResponse data is not CustomProtocolReq")
		return nil
	}
	return request.BasicData
}

func (adapter *CustomStreamProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	request, ok := adapter.protocol.Response.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomStreamProtocolAdapter InitResponse data is not CustomProtocolReq")
		return nil
	}
	return request.BasicData
}

func (adapter *CustomStreamProtocolAdapter) InitResponse(basicData *pb.BasicData, data interface{}) error {
	pid, ok := data.(string)
	if !ok {
		tvLog.Logger.Errorf("CustomStreamProtocolAdapter InitResponse: data is not CustomProtocolReq")
		return nil
	}
	response := &pb.CustomProtocolRes{
		BasicData: basicData,
		PID:       pid,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	adapter.protocol.Response = response
	return nil
}

func (adapter *CustomStreamProtocolAdapter) SetResponseSig(sig []byte) error {
	response, ok := adapter.protocol.Response.(*pb.CustomProtocolRes)
	if !ok {
		return errors.New("failed to cast request to *pb.ReleaseMailboxRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *CustomStreamProtocolAdapter) CallRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomStreamProtocolRequest(adapter.protocol.Request)
	return data, err
}

func (adapter *CustomStreamProtocolAdapter) CallResponseCallback() (interface{}, error) {
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
