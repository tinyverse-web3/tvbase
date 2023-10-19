package protocol

import (
	"context"

	"github.com/libp2p/go-libp2p/core/protocol"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ProtocolHandle interface {
	HandleRequestData(protocolData []byte, moreList ...any) error
	HandleResponseData(protocolData []byte, moreList ...any) error
}

type DmsgService interface {
	GetUserPubkeyHex() (string, error)
	GetProxyPubkey() string
	GetProxyReqPubkey() string
	GetUserSig(protoData []byte) ([]byte, error)
	GetPublishTarget(pubkey string) (*dmsgUser.Target, error)
	PublishProtocol(
		ctx context.Context,
		target *dmsgUser.Target,
		pid pb.PID,
		protoData []byte) error
}

type MailboxSpCallback interface {
	OnCreateMailboxRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, bool, error)
	OnCreateMailboxResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
	OnReadMailboxRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, bool, error)
	OnReadMailboxResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
	OnReleaseMailboxRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, bool, error)
	OnReleaseMailboxResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
}

type MailboxPpCallback interface {
	OnSeekMailboxRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, bool, error)
	OnSeekMailboxResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
}

type QueryPeerCallback interface {
	OnQueryPeerRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, bool, error)
}

type CreatePubsubSpCallback interface {
	OnCreatePubsubRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, bool, error)
	OnCreatePubsubResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
}

type PubsubMsgCallback interface {
	OnPubsubMsgRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, bool, error)
	OnPubsubMsgResponse(
		requestProtoMsg protoreflect.ProtoMessage,
		responseProtoMsg protoreflect.ProtoMessage) (any, error)
}

type CustomSpCallback interface {
	OnCustomRequest(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, bool, error)
	OnCustomResponse(
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
		moreDataList ...any) (protoreflect.ProtoMessage, error)
	InitResponse(
		requestProtoData protoreflect.ProtoMessage,
		basicData *pb.BasicData,
		moreDataList ...any) (protoreflect.ProtoMessage, error)
	CallRequestCallback(
		requestProtoMsg protoreflect.ProtoMessage) (any, any, bool, error)
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
