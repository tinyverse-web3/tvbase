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
	InitRequest(basicData *pb.BasicData, dataList ...any) error
	InitResponse(basicData *pb.BasicData, dataList ...any) error
	GetRequestPID() pb.PID
	GetResponsePID() pb.PID
	GetRequestBasicData() *pb.BasicData
	GetResponseBasicData() *pb.BasicData
	GetResponseRetCode() *pb.RetCode
	SetRequestSig(sig []byte) error
	SetResponseSig(sig []byte) error
	CallRequestCallback() (interface{}, error)
	CallResponseCallback(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
}

type StreamProtocolCallback interface {
	OnCreateMailboxResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
	OnReadMailboxMsgResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
	OnReleaseMailboxResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
	OnCustomStreamProtocolResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
}

type StreamProtocolAdapter interface {
	ProtocolAdapter
	GetStreamRequestPID() protocol.ID
	GetStreamResponsePID() protocol.ID
}

type Protocol struct {
	Ctx              context.Context
	Host             host.Host
	RequestInfoList  map[string]*RequestInfo
	Service          ProtocolService
	RequestProtoMsg  protoreflect.ProtoMessage
	ResponseProtoMsg protoreflect.ProtoMessage
	Adapter          ProtocolAdapter
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
	OnSeekMailboxResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
	OnSendMsgRequest(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
	OnSendMsgResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
	OnCustomPubsubProtocolRequest(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
	OnCustomPubsubProtocolResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
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
