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

func (adapter *ReadMailboxMsgProtocolAdapter) InitResponse(basicData *pb.BasicData, data interface{}) error {
	contentList, ok := data.([]*pb.MailboxItem)
	if !ok {
		return errors.New("fail to cast mailboxMsgDatas to []*pb.MailboxMsgData")
	}
	response := &pb.ReadMailboxRes{
		BasicData:   basicData,
		RetCode:     dmsgProtocol.NewSuccRetCode(),
		ContentList: contentList,
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

func (adapter *ReadMailboxMsgProtocolAdapter) CallRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnReadMailboxMsgRequest(adapter.protocol.Request)
	return data, err
}

func NewReadMailboxMsgProtocol(ctx context.Context, host host.Host, protocolService common.ProtocolService, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewReadMailboxMsgProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
