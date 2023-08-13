package protocol

import (
	"github.com/libp2p/go-libp2p/core/protocol"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ProtocolHandle interface {
	HandleRequestData(
		protocolData []byte,
		moreList ...any) error
}

type DmsgServiceInterface interface {
	GetUserPubkeyHex() (string, error)
	GetUserSig(protoData []byte) ([]byte, error)
	GetPublishTarget(pubkey string) *dmsgUser.Target
	PublishProtocol(
		target *dmsgUser.Target,
		pid pb.PID,
		protoData []byte) error
	IsEnableService() bool
}

type MailboxSpCallback interface {
	OnCreateMailboxRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, error)
	OnCreateMailboxResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
	OnReadMailboxMsgRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, error)
	OnReadMailboxMsgResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
	OnReleaseMailboxRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, error)
	OnReleaseMailboxResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
}

type MailboxPpCallback interface {
	OnSeekMailboxRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, error)
	OnSeekMailboxResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
}

type MsgSpCallback interface {
	OnCustomStreamProtocolRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, error)
	OnCustomStreamProtocolResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
	OnCreateChannelRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, error)
	OnCreateChannelResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
}

type MsgPpCallback interface {
	OnSendMsgRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, error)
	OnSendMsgResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
}

type Adapter interface {
	GetRequestPID() pb.PID
	GetResponsePID() pb.PID
	GetEmptyRequest() protoreflect.ProtoMessage
	GetEmptyResponse() protoreflect.ProtoMessage
	InitRequest(
		basicData *pb.BasicData,
		moreList ...any) (protoreflect.ProtoMessage, error)
	InitResponse(
		requestProtoData protoreflect.ProtoMessage,
		basicData *pb.BasicData,
		moreList ...any) (protoreflect.ProtoMessage, error)
	GetRequestBasicData(
		requestProtoMsg protoreflect.ProtoMessage) *pb.BasicData
	GetResponseBasicData(
		responseProtoMsg protoreflect.ProtoMessage) *pb.BasicData
	GetResponseRetCode(
		responseProtoMsg protoreflect.ProtoMessage) *pb.RetCode
	SetResponseRetCode(
		responseProtoMsg protoreflect.ProtoMessage,
		code int32, result string)
	SetRequestSig(
		requestProtoMsg protoreflect.ProtoMessage,
		sig []byte) error
	SetResponseSig(
		responseProtoMsg protoreflect.ProtoMessage,
		sig []byte) error
	CallRequestCallback(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, error)
	CallResponseCallback(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
}

type SpAdapter interface {
	Adapter
	GetStreamRequestPID() protocol.ID
	GetStreamResponsePID() protocol.ID
}

type PpAdapter interface {
	Adapter
}
