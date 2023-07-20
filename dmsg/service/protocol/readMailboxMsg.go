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
	adapter.protocol.ProtocolRequest = &pb.ReadMailboxMsgReq{}
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetResponseProtocolID() pb.ProtocolID {
	return pb.ProtocolID_READ_MAILBOX_MSG_RES
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetStreamResponseProtocolID() protocol.ID {
	return dmsgProtocol.PidReadMailboxMsgRes
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetStreamRequestProtocolID() protocol.ID {
	return dmsgProtocol.PidReadMailboxMsgReq
}

func (adapter *ReadMailboxMsgProtocolAdapter) DestoryProtocol() {
	adapter.protocol.Host.RemoveStreamHandler(dmsgProtocol.PidReadMailboxMsgReq)
}

func (adapter *ReadMailboxMsgProtocolAdapter) SetProtocolResponseFailRet(errMsg string) {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.ReadMailboxMsgRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewFailRetCode(errMsg)
}

func (adapter *ReadMailboxMsgProtocolAdapter) SetProtocolResponseRet(code int32, result string) {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.ReadMailboxMsgRes)
	if !ok {
		return
	}
	request.RetCode = dmsgProtocol.NewRetCode(code, result)
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetProtocolRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.ReadMailboxMsgReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *ReadMailboxMsgProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolResponse.(*pb.ReadMailboxMsgRes)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *ReadMailboxMsgProtocolAdapter) InitProtocolResponse(basicData *pb.BasicData, data interface{}) error {
	mailboxMsgDatas, ok := data.([]*pb.MailboxMsgData)
	if !ok {
		return errors.New("fail to cast mailboxMsgDatas to []*pb.MailboxMsgData")
	}
	response := &pb.ReadMailboxMsgRes{
		BasicData:       basicData,
		RetCode:         dmsgProtocol.NewSuccRetCode(),
		MailboxMsgDatas: mailboxMsgDatas,
	}
	adapter.protocol.ProtocolResponse = response
	return nil
}

func (adapter *ReadMailboxMsgProtocolAdapter) SetProtocolResponseSign(signature []byte) error {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.ReadMailboxMsgRes)
	if !ok {
		return errors.New("failed to cast request to *pb.ReadMailboxMsgRes")
	}
	response.BasicData.Sign = signature
	return nil
}

func (adapter *ReadMailboxMsgProtocolAdapter) CallProtocolRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnReadMailboxMsgRequest(adapter.protocol.ProtocolRequest)
	return data, err
}

func NewReadMailboxMsgProtocol(ctx context.Context, host host.Host, protocolService common.ProtocolService, protocolCallback common.StreamProtocolCallback) *common.StreamProtocol {
	adapter := NewReadMailboxMsgProtocolAdapter()
	protocol := common.NewStreamProtocol(ctx, host, protocolService, protocolCallback, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
