package stream

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	basicAdapter "github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/basic"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/adapter/util"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/basic"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CustomStreamProtocolAdapter struct {
	basicAdapter.AbstructProtocolAdapter
	protocol *basic.CustomSProtocol
	pid      string
}

func NewCustomStreamProtocolAdapter() *CustomStreamProtocolAdapter {
	ret := &CustomStreamProtocolAdapter{}
	return ret
}

func (adapter *CustomStreamProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_CUSTOM_STREAM_RES
}

func (adapter *CustomStreamProtocolAdapter) GetRequestPID() pb.PID {
	return pb.PID_CUSTOM_STREAM_REQ
}

func (adapter *CustomStreamProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return protocol.ID(common.PidCustomProtocolReq + "/" + adapter.pid)
}

func (adapter *CustomStreamProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return protocol.ID(common.PidCustomProtocolRes + "/" + adapter.pid)
}

func (adapter *CustomStreamProtocolAdapter) GetEmptyRequest() protoreflect.ProtoMessage {
	return &pb.CustomProtocolReq{}
}
func (adapter *CustomStreamProtocolAdapter) GetEmptyResponse() protoreflect.ProtoMessage {
	return &pb.CustomProtocolRes{}
}

func (adapter *CustomStreamProtocolAdapter) InitRequest(
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	requestProtoMsg := &pb.CustomProtocolReq{
		BasicData: basicData,
	}
	if len(dataList) == 2 {
		pid, ok := dataList[0].(string)
		if !ok {
			return nil, errors.New("CustomStreamProtocolAdapter->InitRequest: failed to cast datalist[0] to string for get customProtocolID")
		}
		content, ok := dataList[1].([]byte)
		if !ok {
			return nil, errors.New("CustomStreamProtocolAdapter->InitRequest: failed to cast datalist[1] to []byte for get content")
		}
		requestProtoMsg.PID = pid
		requestProtoMsg.Content = content
	} else {
		return requestProtoMsg, errors.New("CustomStreamProtocolAdapter->InitRequest: parameter dataList need contain customProtocolID and content")
	}
	return requestProtoMsg, nil
}

func (adapter *CustomStreamProtocolAdapter) InitResponse(
	requestProtoData protoreflect.ProtoMessage,
	basicData *pb.BasicData,
	dataList ...any) (protoreflect.ProtoMessage, error) {
	retCode, err := util.GetRetCode(dataList)
	if err != nil {
		return nil, err
	}
	response := &pb.CustomProtocolRes{
		BasicData: basicData,
		RetCode:   retCode,
	}

	request, ok := requestProtoData.(*pb.CustomProtocolReq)
	if !ok {
		return response, fmt.Errorf("CustomStreamProtocolAdapter->InitResponse: fail to cast requestProtoData to *pb.CustomContentReq")
	}
	response.PID = request.PID

	if len(dataList) < 1 {
		return nil, errors.New("CustomStreamProtocolAdapter:InitResponse: dataList need contain customStreamProtocolResponseParam")
	}
	content, ok := dataList[0].([]byte)
	if !ok {
		return response, fmt.Errorf("CustomStreamProtocolAdapter->InitResponse: fail to cast dataList[0](response) to content([]byte)")
	}
	response.Content = content
	return response, nil
}

func (adapter *CustomStreamProtocolAdapter) CallRequestCallback(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	data, retCode, abort, err := adapter.protocol.Callback.OnCustomRequest(requestProtoData)
	return data, retCode, abort, err
}

func (adapter *CustomStreamProtocolAdapter) CallResponseCallback(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	data, err := adapter.protocol.Callback.OnCustomResponse(requestProtoData, responseProtoData)
	return data, err
}

func NewCustomStreamProtocol(
	ctx context.Context,
	host host.Host,
	pid string,
	callbck common.CustomSpCallback,
	service common.DmsgService,
	enableRequest bool,
	pubkey string,
) *basic.CustomSProtocol {
	ret := NewCustomStreamProtocolAdapter()
	ret.pid = pid
	protocol := basic.NewCustomSProtocol(ctx, host, callbck, service, ret, enableRequest, pubkey)
	ret.protocol = protocol
	return protocol
}
