package common

import (
	"context"
	"crypto/ecdsa"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ProtocolService interface {
	GetUserPubkeyHex() (string, error)
	GetUserSig(protoData []byte) ([]byte, error)
	RegPubsubProtocolResCallback(protocolID pb.PID, subscribe dmsgProtocol.ResSubscribe) error
	RegPubsubProtocolReqCallback(protocolID pb.PID, subscribe dmsgProtocol.ReqSubscribe) error
	PublishProtocol(ctx context.Context, userPubkey string, protocolID pb.PID, protocolData []byte) error
}

type StreamProtocolCallback interface {
	OnCreateMailboxRequest(protoreflect.ProtoMessage) (any, any, error)
	OnCreateMailboxResponse(protoreflect.ProtoMessage) (any, error)
	OnReadMailboxMsgRequest(protoreflect.ProtoMessage) (any, any, error)
	OnReleaseMailboxRequest(protoreflect.ProtoMessage) (any, any, error)
	OnCreatePubChannelRequest(protoreflect.ProtoMessage) (any, any, error)
	OnCustomStreamProtocolRequest(protoreflect.ProtoMessage) (any, any, error)
	OnCustomStreamProtocolResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (any, error)
}

type StreamProtocolAdapter interface {
	GetResponsePID() pb.PID
	GetStreamResponsePID() protocol.ID
	GetStreamRequestPID() protocol.ID
	GetEmptyRequest() protoreflect.ProtoMessage
	GetEmptyResponse() protoreflect.ProtoMessage
	GetRequestBasicData(requestProtoData protoreflect.ProtoMessage) *pb.BasicData
	GetResponseBasicData(responseProtoData protoreflect.ProtoMessage) *pb.BasicData
	InitResponse(
		requestProtoData protoreflect.ProtoMessage,
		basicData *pb.BasicData,
		dataList ...any) (protoreflect.ProtoMessage, error)
	SetResponseSig(responseProtoData protoreflect.ProtoMessage, sig []byte) error
	CallRequestCallback(requestProtoData protoreflect.ProtoMessage) (any, any, error)
	CallResponseCallback(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error)
}
type StreamProtocol struct {
	Ctx      context.Context
	Host     host.Host
	Service  ProtocolService
	Callback StreamProtocolCallback
	Adapter  StreamProtocolAdapter
}

// pubsubProtocol
type PubsubProtocolAdapter interface {
	GetRequestPID() pb.PID
	GetResponsePID() pb.PID
	GetEmptyRequest() protoreflect.ProtoMessage
	GetEmptyResponse() protoreflect.ProtoMessage
	GetRequestBasicData(requestProtoData protoreflect.ProtoMessage) *pb.BasicData
	GetResponseBasicData(responseProtoData protoreflect.ProtoMessage) *pb.BasicData
	InitResponse(
		requestProtoData protoreflect.ProtoMessage,
		basicData *pb.BasicData,
		dataList ...any) (protoreflect.ProtoMessage, error)
	SetResponseSig(responseProtoData protoreflect.ProtoMessage, sig []byte) error
	CallRequestCallback(requestProtoData protoreflect.ProtoMessage) (any, any, error)
}

type PubsubProtocolCallback interface {
	OnSendMsgRequest(protoreflect.ProtoMessage) (any, error)
	OnCustomPubsubProtocolRequest(protoreflect.ProtoMessage) (any, any, error)
	OnCustomPubsubProtocolResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (any, error)
}

type PubsubProtocol struct {
	Ctx      context.Context
	Host     host.Host
	Service  ProtocolService
	Callback PubsubProtocolCallback
	Adapter  PubsubProtocolAdapter
}

type UserPubsub struct {
	Topic        *pubsub.Topic
	CancelFunc   context.CancelFunc
	Subscription *pubsub.Subscription
}

type DestUserInfo struct {
	UserPubsub
	MsgRWMutex          sync.RWMutex
	LastReciveTimestamp int64
}

type PubChannelInfo struct {
	UserPubsub
	LastReciveTimestamp int64
}

type CustomProtocolPubsub struct {
	UserPubsub
}

type CustomStreamProtocolInfo struct {
	Service  customProtocol.CustomStreamProtocolService
	Protocol *StreamProtocol
}

type CustomPubsubProtocolInfo struct {
	Service  customProtocol.CustomPubsubProtocolService
	Protocol *PubsubProtocol
}

const MailboxAlreadyExistCode = 1
const PubChannelAlreadyExistCode = 1

type UserInfo struct {
	UserKey *UserKey
}
type UserKey struct {
	PubKeyHex string
	PriKeyHex string
	PubKey    *ecdsa.PublicKey
	PriKey    *ecdsa.PrivateKey
}
