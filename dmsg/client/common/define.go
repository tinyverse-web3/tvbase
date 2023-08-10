package common

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ProtocolService interface {
	GetUserPubkeyHex() (string, error)
	GetUserSig(protoData []byte) ([]byte, error)
	PublishProtocol(userPubkey string, pid pb.PID, protoData []byte) error
}

type RequestInfo struct {
	ProtoMessage    protoreflect.ProtoMessage
	CreateTimestamp int64
	DoneChan        chan any
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

type StreamProtocolAdapter interface {
	ProtocolAdapter
	GetStreamRequestPID() protocol.ID
	GetStreamResponsePID() protocol.ID
}

type Protocol struct {
	Ctx             context.Context
	Host            host.Host
	RequestInfoList map[string]*RequestInfo
	Service         ProtocolService
	Adapter         ProtocolAdapter
}

type StreamProtocol struct {
	Protocol
	Callback StreamProtocolCallback
	stream   network.Stream
}

type PubsubProtocolAdapter interface {
	ProtocolAdapter
}

type PubsubProtocol struct {
	Protocol
	Callback PubsubProtocolCallback
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

type UserMsg struct {
	ID             string
	SrcUserPubkey  string
	DestUserPubkey string
	Direction      string
	TimeStamp      int64
	MsgContent     string
}

type CustomStreamProtocolInfo struct {
	Client   customProtocol.CustomStreamProtocolClient
	Protocol *StreamProtocol
}

type CustomPubsubProtocolInfo struct {
	Client   customProtocol.CustomPubsubProtocolClient
	Protocol *PubsubProtocol
}

// service

const MailboxLimitErr = "mailbox is limited"
const MailboxAlreadyExistErr = "dest pubkey already exists"
const MailboxAlreadyExistCode = 1
