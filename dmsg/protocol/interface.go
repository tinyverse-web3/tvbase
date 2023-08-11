package protocol

import (
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ReqSubscribe interface {
	HandleRequestData(protocolData []byte, dataList ...any) error
}

type ResSubscribe interface {
	HandleResponseData(protocolData []byte, dataList ...any) error
}

type ProtocolService interface {
	GetUserPubkeyHex() (string, error)
	GetUserSig(protoData []byte) ([]byte, error)
	PublishProtocol(pubkey string, pid pb.PID, protoData []byte) error
}

type StreamProtocolCallback interface {
	OnCreateMailboxRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, error)
	OnCreateMailboxResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
	OnReadMailboxMsgRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, error)
	OnReadMailboxMsgResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
	OnReleaseMailboxRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, error)
	OnReleaseMailboxResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
	OnCustomStreamProtocolRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, error)
	OnCustomStreamProtocolResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
	OnCreateChannelRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, error)
	OnCreateChannelResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
}

type PubsubProtocolCallback interface {
	OnSeekMailboxRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, error)
	OnSeekMailboxResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
	OnQueryPeerRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, error)
	OnQueryPeerResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
	OnSendMsgRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, error)
	OnSendMsgResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
	OnCustomPubsubProtocolRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, error)
	OnCustomPubsubProtocolResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
}

type ProtocolAdapter interface {
	GetRequestPID() pb.PID
	GetResponsePID() pb.PID
	GetEmptyRequest() protoreflect.ProtoMessage
	GetEmptyResponse() protoreflect.ProtoMessage
	InitRequest(basicData *pb.BasicData, dataList ...any) (protoreflect.ProtoMessage, error)
	InitResponse(basicData *pb.BasicData, dataList ...any) (protoreflect.ProtoMessage, error)
	GetRequestBasicData(requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData
	GetResponseBasicData(responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData
	GetResponseRetCode(responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode
	SetResponseRetCode(responseProtoMsg protoreflect.ProtoMessage, code int32, result string)
	SetRequestSig(requestProtoMsg protoreflect.ProtoMessage, sig []byte) error
	SetResponseSig(responseProtoMsg protoreflect.ProtoMessage, sig []byte) error
	CallRequestCallback(requestProtoMsg protoreflect.ProtoMessage) (any, error)
	CallResponseCallback(requestProtoMsg protoreflect.ProtoMessage, responseProtoMsg protoreflect.ProtoMessage) (any, error)
}

type StreamProtocolAdapter interface {
	ProtocolAdapter
	GetStreamRequestPID() protocol.ID
	GetStreamResponsePID() protocol.ID
}

type PubsubProtocolAdapter interface {
	ProtocolAdapter
}
