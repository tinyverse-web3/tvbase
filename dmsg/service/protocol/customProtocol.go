package protocol

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
)

type CustomProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
	pid      string
}

func NewCustomProtocolAdapter() *CustomProtocolAdapter {
	ret := &CustomProtocolAdapter{}
	return ret
}

func (adapter *CustomProtocolAdapter) init(customProtocolId string) {
	adapter.pid = customProtocolId
	adapter.protocol.Host.SetStreamHandler(protocol.ID(dmsgProtocol.PidCustomProtocolReq+"/"+adapter.pid), adapter.protocol.OnRequest)
	adapter.protocol.ProtocolRequest = &pb.CustomProtocolReq{}
	adapter.protocol.ProtocolResponse = &pb.CustomProtocolRes{}
}

func (adapter *CustomProtocolAdapter) GetResponseProtocolID() pb.ProtocolID {
	return pb.ProtocolID_CUSTOM_PROTOCOL_RES
}

func (adapter *CustomProtocolAdapter) GetStreamResponseProtocolID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolRes + "/" + adapter.pid)
}

func (adapter *CustomProtocolAdapter) DestoryProtocol() {
	adapter.protocol.Host.RemoveStreamHandler(protocol.ID(dmsgProtocol.PidCustomProtocolReq + "/" + adapter.pid))
}

func (adapter *CustomProtocolAdapter) SetProtocolResponseFailRet(errMsg string) {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter InitProtocolResponse data is not CustomProtocolReq")
		return
	}
	request.RetCode = dmsgProtocol.NewFailRetCode(errMsg)
}

func (adapter *CustomProtocolAdapter) SetProtocolResponseRet(code int32, result string) {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter InitProtocolResponse data is not CustomProtocolReq")
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *CustomProtocolAdapter) GetProtocolRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.CustomProtocolReq)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter InitProtocolResponse data is not CustomProtocolReq")
		return nil
	}
	return request.BasicData
}

func (adapter *CustomProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter InitProtocolResponse data is not CustomProtocolReq")
		return nil
	}
	return request.BasicData
}

func (adapter *CustomProtocolAdapter) InitProtocolResponse(basicData *pb.BasicData, data interface{}) error {
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

func (adapter *CustomProtocolAdapter) SetProtocolResponseSign(signature []byte) {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		return
	}
	response.BasicData.Sign = signature
}

func (adapter *CustomProtocolAdapter) CallProtocolRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomProtocolRequest(adapter.protocol.ProtocolRequest)
	return data, err
}

func (adapter *CustomProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomProtocolResponse(adapter.protocol.ProtocolRequest, adapter.protocol.ProtocolResponse)
	return data, err
}

func NewCustomProtocol(ctx context.Context, host host.Host, customProtocolId string, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewCustomProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init(customProtocolId)
	return protocol
}
