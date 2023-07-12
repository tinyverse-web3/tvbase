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
	// TODO
	adapter.protocol.ProtocolRequest = &pb.CustomProtocolReq{}
	adapter.protocol.ProtocolResponse = &pb.CustomProtocolRes{}
}

func (adapter *CustomPubsubProtocolAdapter) GetResponseProtocolID() pb.ProtocolID {
	return pb.ProtocolID_CUSTOM_STREAM_PROTOCOL_RES
}

func (adapter *CustomPubsubProtocolAdapter) GetRequestProtocolID() pb.ProtocolID {
	return pb.ProtocolID_CUSTOM_STREAM_PROTOCOL_REQ
}

func (adapter *CustomPubsubProtocolAdapter) GetStreamResponseProtocolID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolRes + "/" + adapter.pid)
}

func (adapter *CustomPubsubProtocolAdapter) DestoryProtocol() {
	adapter.protocol.Host.RemoveStreamHandler(protocol.ID(dmsgProtocol.PidCustomProtocolReq + "/" + adapter.pid))
}

func (adapter *CustomPubsubProtocolAdapter) SetProtocolResponseFailRet(errMsg string) {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter->SetProtocolResponseFailRet: data is not CustomProtocolReq")
		return
	}
	request.RetCode = dmsgProtocol.NewFailRetCode(errMsg)
}

func (adapter *CustomPubsubProtocolAdapter) SetProtocolResponseRet(code int32, result string) {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter->SetProtocolResponseRet: data is not CustomProtocolReq")
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *CustomPubsubProtocolAdapter) GetProtocolRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.CustomProtocolReq)
	if !ok {
		tvLog.Logger.Errorf("CustomProtocolAdapter GetProtocolRequestBasicData data is not CustomProtocolReq")
		return nil
	}
	return request.BasicData
}

func (adapter *CustomPubsubProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		tvLog.Logger.Errorf("CustomPubsubProtocolAdapter->GetProtocolResponseBasicData: data is not CustomProtocolReq")
		return nil
	}
	return request.BasicData
}

func (adapter *CustomPubsubProtocolAdapter) InitProtocolResponse(basicData *pb.BasicData, data interface{}) error {
	response := &pb.CustomProtocolRes{
		BasicData: basicData,
		RetCode:   dmsgProtocol.NewSuccRetCode(),
	}
	request, ok := data.(*pb.CustomProtocolReq)
	if !ok {
		tvLog.Logger.Errorf("CustomPubsubProtocolAdapter->InitProtocolResponse: data is not CustomProtocolReq")
		return nil
	}
	response.CustomProtocolID = request.CustomProtocolID
	adapter.protocol.ProtocolResponse = response
	return nil
}

func (adapter *CustomPubsubProtocolAdapter) SetProtocolResponseSign(signature []byte) error {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		return errors.New("CustomPubsubProtocolAdapter->SetProtocolResponseSign: failed to cast request to *pb.ReleaseMailboxRes")
	}
	response.BasicData.Sign = signature
	return nil
}

func (adapter *CustomPubsubProtocolAdapter) CallProtocolRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomPubsubProtocolRequest(adapter.protocol.ProtocolRequest)
	return data, err
}

func (adapter *CustomPubsubProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomPubsubProtocolResponse(adapter.protocol.ProtocolRequest, adapter.protocol.ProtocolResponse)
	return data, err
}

func (adapter *CustomPubsubProtocolAdapter) GetProtocolResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
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
