package protocol

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"

	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
)

type ReadMailboxMsgProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.StreamProtocol
}

func NewReadMailboxMsgProtocolAdapter() *ReadMailboxMsgProtocolAdapter {
	ret := &ReadMailboxMsgProtocolAdapter{}
	return ret
}

func (adapter *ReadMailboxMsgProtocolAdapter) init() {
	adapter.protocol.Request = &pb.ReadMailboxReq{}
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetResponsePID() pb.PID {
	return pb.PID_READ_MAILBOX_RES
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetStreamResponsePID() protocol.ID {
	return dmsgProtocol.PidReadMailboxMsgRes
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetStreamRequestPID() protocol.ID {
	return dmsgProtocol.PidReadMailboxMsgReq
}

func (adapter *ReadMailboxMsgProtocolAdapter) DestoryProtocol() {
	adapter.protocol.Host.RemoveStreamHandler(dmsgProtocol.PidReadMailboxMsgReq)
}

func (adapter *ReadMailboxMsgProtocolAdapter) SetProtocolResponseFailRet(errMsg string) {
	request, ok := adapter.protocol.Response.(*pb.ReadMailboxRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewFailRetCode(errMsg)
}

func (adapter *ReadMailboxMsgProtocolAdapter) SetProtocolResponseRet(code int32, result string) {
	request, ok := adapter.protocol.Response.(*pb.ReadMailboxRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.Request.(*pb.ReadMailboxReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetResponseBasicData() *pb.BasicData {
	request, ok := adapter.protocol.Response.(*pb.ReadMailboxRes)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *ReadMailboxMsgProtocolAdapter) InitResponse(basicData *pb.BasicData, dataList ...any) error {
	if len(dataList) < 1 {
		return errors.New("ReadMailboxMsgProtocolAdapter:InitResponse: dataList need contain MailboxItemList")
	}
	mailboxItemList, ok := dataList[0].([]*pb.MailboxItem)
	if !ok {
		return errors.New("ReadMailboxMsgProtocolAdapter:InitResponse: fail to cast dataList[0] to []*pb.MailboxMsgData")
	}
	retCode := dmsgProtocol.NewSuccRetCode()
	if len(dataList) > 1 {
		retCode = dataList[1].(*pb.RetCode)
	}
	response := &pb.ReadMailboxRes{
		BasicData:   basicData,
		RetCode:     retCode,
		ContentList: mailboxItemList,
	}
	adapter.protocol.Response = response
	return nil
}

func (adapter *ReadMailboxMsgProtocolAdapter) SetResponseSig(sig []byte) error {
	response, ok := adapter.protocol.Response.(*pb.ReadMailboxRes)
	if !ok {
		return errors.New("failed to cast request to *pb.ReadMailboxMsgRes")
	}
	response.BasicData.Sig = sig
	return nil
}

func (adapter *ReadMailboxMsgProtocolAdapter) CallRequestCallback() (any, any, error) {
	data, retCode, err := adapter.protocol.Callback.OnReadMailboxMsgRequest(adapter.protocol.Request)
	return data, retCode, err
}

func NewReadMailboxMsgProtocol(ctx context.Context, host host.Host, protocolService common.ProtocolService, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewReadMailboxMsgProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
