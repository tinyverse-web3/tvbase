package common

import (
	"context"
	"crypto/ecdsa"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type RequestInfo struct {
	ProtoMessage    protoreflect.ProtoMessage
	CreateTimestamp int64
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

type ProtocolService interface {
	GetCurSrcUserPubKeyHex() string
	GetCurSrcUserSig(protoData []byte) ([]byte, error)
	PublishProtocol(userPubkey string, pid pb.PID, protoData []byte) error
}

type UserPubsub struct {
	Topic           *pubsub.Topic
	Subscription    *pubsub.Subscription
	IsReadPubsubMsg bool
	CancelFunc      context.CancelFunc
}
type SrcUserInfo struct {
	UserPubsub
	MailboxPeerID       string
	MailboxCreateSignal chan bool
	UserKey             *SrcUserKey
	GetSigCallback      GetSigCallback
}

type DestUserInfo struct {
	UserPubsub
}

type SrcUserKey struct {
	PubkeyHex string
	Pubkey    *ecdsa.PublicKey
}

type OnReceiveMsg func(srcUserPubkey string, destUserPubkey string, msgContent []byte, timeStamp int64, msgID string, direction string)

type UserMsg struct {
	ID             string
	SrcUserPubkey  string
	DestUserPubkey string
	Direction      string
	TimeStamp      int64
	MsgContent     string
}

type GetSigCallback func(protoData []byte) (sig []byte, err error)

type CustomStreamProtocolInfo struct {
	Client   customProtocol.CustomStreamProtocolClient
	Protocol *StreamProtocol
}

type CustomPubsubProtocolInfo struct {
	Client   customProtocol.CustomPubsubProtocolClient
	Protocol *PubsubProtocol
}
