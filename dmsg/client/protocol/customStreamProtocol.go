package protocol

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
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

func (adapter *CustomStreamProtocolAdapter) init(customProtocolId string) {
	adapter.pid = customProtocolId
	adapter.protocol.Host.SetStreamHandler(protocol.ID(dmsgProtocol.PidCustomProtocolRes+"/"+adapter.pid), adapter.protocol.OnResponse)
	adapter.protocol.ProtocolRequest = &pb.CustomProtocolReq{}
	adapter.protocol.ProtocolResponse = &pb.CustomProtocolRes{}
}

func (adapter *CustomStreamProtocolAdapter) GetResponseProtocolID() pb.ProtocolID {
	return pb.ProtocolID_CUSTOM_STREAM_PROTOCOL_RES
}

func (adapter *CustomStreamProtocolAdapter) GetRequestProtocolID() pb.ProtocolID {
	return pb.ProtocolID_CUSTOM_STREAM_PROTOCOL_REQ
}

func (adapter *CustomStreamProtocolAdapter) GetStreamRequestProtocolID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolReq + "/" + adapter.pid)
}

func (adapter *CustomStreamProtocolAdapter) InitProtocolRequest(basicData *pb.BasicData, dataList ...any) error {
	if len(dataList) == 2 {
		customProtocolID, ok := dataList[0].(string)
		if !ok {
			return errors.New("CustomStreamProtocolAdapter->InitProtocolRequest: failed to cast datalist[0] to string for get customProtocolID")
		}
		content, ok := dataList[1].([]byte)
		if !ok {
			return errors.New("CustomStreamProtocolAdapter->InitProtocolRequest: failed to cast datalist[1] to []byte for get content")
		}
		adapter.protocol.ProtocolRequest = &pb.CustomProtocolReq{
			BasicData:        basicData,
			CustomProtocolID: customProtocolID,
			Content:          content,
		}
	} else {
		return errors.New("CustomStreamProtocolAdapter->InitProtocolRequest: parameter dataList need contain customProtocolID and content")
	}
	return nil
}

func (adapter *CustomStreamProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomStreamProtocolResponse(adapter.protocol.ProtocolRequest, adapter.protocol.ProtocolResponse)
	return data, err
}

func (adapter *CustomStreamProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CustomStreamProtocolAdapter) GetProtocolResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *CustomStreamProtocolAdapter) SetProtocolRequestSign(signature []byte) {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.CustomProtocolReq)
	if !ok {
		return
	}
	request.BasicData.Sign = signature
}

func NewCustomStreamProtocol(
	ctx context.Context,
	host host.Host,
	customProtocolId string,
	protocolCallback common.StreamProtocolCallback,
	protocolService common.ProtocolService,
) *common.StreamProtocol {
	ret := NewCustomStreamProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolCallback, protocolService, ret)
	ret.protocol = protocol
	ret.init(customProtocolId)
	return protocol
}
