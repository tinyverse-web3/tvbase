package protocol

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
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
	adapter.protocol.Host.SetStreamHandler(protocol.ID(dmsgProtocol.PidCustomProtocolRes+"/"+adapter.pid), adapter.protocol.OnResponse)
	adapter.protocol.ProtocolRequest = &pb.CustomProtocolReq{}
	adapter.protocol.ProtocolResponse = &pb.CustomProtocolRes{}
}

func (adapter *CustomProtocolAdapter) GetResponseProtocolID() pb.ProtocolID {
	return pb.ProtocolID_CUSTOM_STREAM_PROTOCOL_RES
}

func (adapter *CustomProtocolAdapter) GetRequestProtocolID() pb.ProtocolID {
	return pb.ProtocolID_CUSTOM_STREAM_PROTOCOL_REQ
}

func (adapter *CustomProtocolAdapter) GetStreamRequestProtocolID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolReq + "/" + adapter.pid)
}

func (adapter *CustomProtocolAdapter) InitProtocolRequest(basicData *pb.BasicData) {
	request := &pb.CustomProtocolReq{
		BasicData: basicData,
	}
	adapter.protocol.ProtocolRequest = request
}

func (adapter *CustomProtocolAdapter) SetCustomContent(customProtocolID string, content []byte) error {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.CustomProtocolReq)
	if !ok {
		dmsgLog.Logger.Error("ProtocolRequest is not CustomContentReq")
		return fmt.Errorf("ProtocolRequest is not CustomContentReq")
	}
	request.CustomProtocolID = customProtocolID
	request.Content = content
	return nil
}

func (adapter *CustomProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomProtocolResponse(adapter.protocol.ProtocolRequest, adapter.protocol.ProtocolResponse)
	return data, err
}

func (adapter *CustomProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CustomProtocolAdapter) GetProtocolResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *CustomProtocolAdapter) SetProtocolRequestSign(signature []byte) {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.CustomProtocolReq)
	if !ok {
		return
	}
	request.BasicData.Sign = signature
}

func NewCustomStreamProtocol(ctx context.Context, host host.Host, customProtocolId string,
	protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewCustomProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init(customProtocolId)
	return protocol
}
