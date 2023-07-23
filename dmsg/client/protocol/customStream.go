package protocol

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"google.golang.org/protobuf/reflect/protoreflect"
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

func (adapter *CustomStreamProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CUSTOM_STREAM_PROTOCOL_RES
}

func (adapter *CustomStreamProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CUSTOM_STREAM_PROTOCOL_REQ
}

func (adapter *CustomStreamProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolReq + "/" + adapter.pid)
}

func (adapter *CustomStreamProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return protocol.ID(dmsgProtocol.PidCustomProtocolRes + "/" + adapter.pid)
}

func (adapter *CustomStreamProtocolAdapter) InitRequest(basicData *pb.BasicData, dataList ...any) error {
	if len(dataList) == 2 {
		pid, ok := dataList[0].(string)
		if !ok {
			return errors.New("CustomStreamProtocolAdapter->InitRequest: failed to cast datalist[0] to string for get customProtocolID")
		}
		content, ok := dataList[1].([]byte)
		if !ok {
			return errors.New("CustomStreamProtocolAdapter->InitRequest: failed to cast datalist[1] to []byte for get content")
		}
		adapter.protocol.ProtocolRequest = &pb.CustomProtocolReq{
			BasicData: basicData,
			PID:       pid,
			Content:   content,
		}
	} else {
		return errors.New("CustomStreamProtocolAdapter->InitRequest: parameter dataList need contain customProtocolID and content")
	}
	return nil
}

func (adapter *CustomStreamProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (interface{}, error) {
	data, err := adapter.protocol.Callback.OnCustomStreamProtocolResponse(requestProtoData, responseProtoData)
	return data, err
}

func (adapter *CustomStreamProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *CustomStreamProtocolAdapter) GetResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.CustomProtocolRes)
	if !ok {
		return nil
	}
	return response.RetCode
}

func (adapter *CustomStreamProtocolAdapter) SetRequestSig(sig []byte) {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.CustomProtocolReq)
	if !ok {
		return
	}
	request.BasicData.Sig = sig
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
	ret.init()
	ret.pid = customProtocolId
	return protocol
}
