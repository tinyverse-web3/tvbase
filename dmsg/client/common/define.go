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

type StreamProtocolCallback interface {
	OnCreateMailboxResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
	OnReadMailboxMsgResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
	OnReleaseMailboxResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
	OnSeekMailboxResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
	OnCustomStreamProtocolResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
}
type StreamProtocolAdapter interface {
	InitRequest(basicData *pb.BasicData, dataList ...any) error
	GetRequestPID() pb.PID
	GetStreamRequestPID() protocol.ID
	GetStreamResponsePID() protocol.ID
	GetResponseBasicData() *pb.BasicData
	GetResponseRetCode() *pb.RetCode
	SetRequestSig(sig []byte)
	CallResponseCallback() (interface{}, error)
}

type RequestInfo struct {
	ProtoMessage    protoreflect.ProtoMessage
	CreateTimestamp int64
}

type StreamProtocol struct {
	Ctx              context.Context
	Host             host.Host
	RequestInfoList  map[string]*RequestInfo
	Callback         StreamProtocolCallback
	ProtocolService  ProtocolService
	ProtocolRequest  protoreflect.ProtoMessage
	ProtocolResponse protoreflect.ProtoMessage
	Adapter          StreamProtocolAdapter
}

type PubsubProtocolAdapter interface {
	InitRequest(basicData *pb.BasicData, dataList ...any) error
	GetRequestPID() pb.PID
	GetResponsePID() pb.PID
	GetRequestBasicData() *pb.BasicData
	GetResponseBasicData() *pb.BasicData
	GetResponseRetCode() *pb.RetCode
	SetRequestSig(sig []byte) error
	CallRequestCallback() (interface{}, error)
	CallResponseCallback() (interface{}, error)
	GetMsgSource() MsgSource
}

type PubsubProtocol struct {
	Ctx              context.Context
	Host             host.Host
	RequestInfoList  map[string]*RequestInfo
	Callback         PubsubProtocolCallback
	ProtocolService  ProtocolService
	ProtocolRequest  protoreflect.ProtoMessage
	ProtocolResponse protoreflect.ProtoMessage
	Adapter          PubsubProtocolAdapter
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
	PublishProtocol(protocolID pb.PID, userPubkey string, protocolData []byte, msgSource MsgSource) error
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

type MsgSource int32

type msgSourceEnum struct {
	DestUser MsgSource
	SrcUser  MsgSource
}

var MsgSourceEnum = msgSourceEnum{
	DestUser: 0,
	SrcUser:  1,
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
