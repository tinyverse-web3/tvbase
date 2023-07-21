package protocol

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/client/common"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
)

type SendMsgProtocolAdapter struct {
	common.CommonProtocolAdapter
	protocol *common.PubsubProtocol
}

func NewSendMsgProtocolAdapter() *SendMsgProtocolAdapter {
	ret := &SendMsgProtocolAdapter{}
	return ret
}

func (adapter *SendMsgProtocolAdapter) init() {
	adapter.protocol.ProtocolRequest = &pb.SendMsgReq{}
	adapter.protocol.ProtocolResponse = &pb.SendMsgRes{}
}

func (adapter *SendMsgProtocolAdapter) GetRequestProtocolID() pb.ProtocolID {
	return pb.ProtocolID_SEND_MSG_REQ
}

func (adapter *SendMsgProtocolAdapter) GetResponseProtocolID() pb.ProtocolID {
	return pb.ProtocolID_SEND_MSG_RES
}

func (adapter *SendMsgProtocolAdapter) GetPubsubSource() common.PubsubSourceType {
	return common.PubsubSource.DestUser
}

func (adapter *SendMsgProtocolAdapter) InitProtocolRequest(basicData *pb.BasicData, dataList ...any) error {
	if len(dataList) == 2 {
		srcPubkey, ok := dataList[0].(string)
		if !ok {
			return errors.New("SendMsgProtocolAdapter->InitProtocolRequest: failed to cast datalist[0] to string for get srcPubkey")
		}
		msgContent, ok := dataList[1].([]byte)
		if !ok {
			return errors.New("SendMsgProtocolAdapter->InitProtocolRequest: failed to cast datalist[1] to []byte for get msgContent")
		}

		adapter.protocol.ProtocolRequest = &pb.SendMsgReq{
			BasicData:  basicData,
			SrcPubkey:  srcPubkey,
			MsgContent: msgContent,
		}
	} else {
		return errors.New("SendMsgProtocolAdapter->InitProtocolRequest: parameter dataList need contain srcPubkey and msgContent")
	}
	return nil
}

func (adapter *SendMsgProtocolAdapter) CallProtocolRequestCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnSendMsgRequest(adapter.protocol.ProtocolRequest)
	return data, err
}

func (adapter *SendMsgProtocolAdapter) CallProtocolResponseCallback() (interface{}, error) {
	data, err := adapter.protocol.Callback.OnSendMsgResponse(adapter.protocol.ProtocolResponse)
	return data, err
}

func (adapter *SendMsgProtocolAdapter) GetProtocolRequestBasicData() *pb.BasicData {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.SendMsgReq)
	if !ok {
		return nil
	}
	return request.BasicData
}

func (adapter *SendMsgProtocolAdapter) GetProtocolResponseBasicData() *pb.BasicData {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.SendMsgRes)
	if !ok {
		return nil
	}
	return response.BasicData
}

func (adapter *SendMsgProtocolAdapter) GetProtocolResponseRetCode() *pb.RetCode {
	response, ok := adapter.protocol.ProtocolResponse.(*pb.SendMsgRes)
	if !ok {
		return nil
	}
	return response.RetCode
}
func (adapter *SendMsgProtocolAdapter) SetProtocolRequestSign(signature []byte) error {
	request, ok := adapter.protocol.ProtocolRequest.(*pb.SendMsgReq)
	if !ok {
		return errors.New("failed to cast request to *pb.SendMsgReq")
	}
	request.BasicData.Sign = signature
	return nil
}

func NewSendMsgProtocol(
	ctx context.Context,
	host host.Host,
	protocolCallback common.PubsubProtocolCallback,
	dmsgService common.ProtocolService) *common.PubsubProtocol {
	adapter := NewSendMsgProtocolAdapter()
	protocol := common.NewPubsubProtocol(ctx, host, protocolCallback, dmsgService, adapter)
	adapter.protocol = protocol
	adapter.init()
	return protocol
}
