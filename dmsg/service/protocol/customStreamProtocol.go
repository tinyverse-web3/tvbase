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
	adapter.protocol.ProtocolRequest = &pb.CustomProtocolReq{}
	adapter.protocol.ProtocolResponse = &pb.CustomProtocolRes{}
}

func (adapter *CustomStreamProtocolAdapter) GetResponseProtocolID() pb.ProtocolID {
	return pb.ProtocolID_CUSTOM_STREAM_PROTOCOL_RES
}

func (adapter *CustomStreamProtocolAdapter) GetRequestProtocolID() pb.ProtocolID {
	return pb.ProtocolID_CUSTOM_STREAM_PROTOCOL_REQ
}

func (adapter *CustomStreamProtocolAdapter) GetStreamResponseProtocolID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolRes + "/" + adapter.pid)
}

func (adapter *CustomStreamProtocolAdapter) GetStreamRequestProtocolID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolReq + "/" + adapter.pid)
}

func (adapter *CustomStreamProtocolAdapter) DestoryProtocol() {
	adapter.protocol.Host.RemoveStreamHandler(protocol.ID(dmsgProtocol.PidCustomProtocolReq + "/" + adapter.pid))
}

func (adapter *CustomStreamProtocolAdapter) SetProtocolResponseFailRet(errMsg string) {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter InitProtocolResponse data is not CustomProtocolReq")
		return
	}
	request.RetCode = dmsgProtocol.NewFailRetCode(errMsg)
}

func (adapter *CustomStreamProtocolAdapter) SetProtocolResponseRet(code int32, result string) {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter InitProtocolResponse data is not CustomProtocolReq")
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *CustomStreamProtocolAdapter) GetProtocolRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.CustomProtocolReq)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter InitProtocolResponse data is not CustomProtocolReq")
		return nil
	}
	return request.BasicData
}

func (adapter *CustomStreamProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter InitProtocolResponse data is not CustomProtocolReq")
		return nil
	}
	return request.BasicData
}

func (adapter *CustomStreamProtocolAdapter) InitProtocolResponse(basicData *pb.BasicData, data interface{}) error {
	response := &pb.CustomProtocolRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	request, ok := data.(*pb.CustomProtocolReq)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter InitProtocolResponse data is not CustomProtocolReq")
		return nil
	}
	response.CustomProtocolID = request.CustomProtocolID
	adapter.protocol.ProtocolResponse = response
	return nil
}

func (adapter *CustomStreamProtocolAdapter) SetProtocolResponseSign(signature []byte) error {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		return errors.New("failed to cast request to *pb.ReleaseMailboxRes")
	}
	response.BasicData.Sign = signature
	return nil
}

func (adapter *CustomStreamProtocolAdapter) CallProtocolRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomStreamProtocolRequest(adapter.protocol.ProtocolRequest)
	return data, err
}

func (adapter *CustomStreamProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomStreamProtocolResponse(adapter.protocol.ProtocolRequest, adapter.protocol.ProtocolResponse)
	return data, err
}

func NewCustomStreamProtocol(
	ctx context.Context,
	host host.Host,
	customProtocolId string,
	protocolService common.ProtocolService,
	protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewCustomStreamProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	adapter.pid = customProtocolId
	return adapter.protocol
}
