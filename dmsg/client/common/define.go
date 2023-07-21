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
	OnCreateMailboxResponse(protoreflect.ProtoMessage) (interface{}, error)
	OnReadMailboxMsgResponse(protoreflect.ProtoMessage) (interface{}, error)
	OnReleaseMailboxResponse(protoreflect.ProtoMessage) (interface{}, error)
	OnSeekMailboxResponse(protoreflect.ProtoMessage) (interface{}, error)
	OnCustomStreamProtocolResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
}
type StreamProtocolAdapter interface {
	InitProtocolRequest(basicData *pb.BasicData, dataList ...any) error
	GetRequestProtocolID() pb.ProtocolID
	GetStreamRequestProtocolID() protocol.ID
	GetStreamResponseProtocolID() protocol.ID
	GetProtocolResponseBasicData() *pb.BasicData
	GetProtocolResponseRetCode() *pb.RetCode
	SetProtocolRequestSign(signature []byte)
	CallProtocolResponseCallback() (interface{}, error)
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
	InitProtocolRequest(basicData *pb.BasicData, dataList ...any) error
	GetRequestProtocolID() pb.ProtocolID
	GetResponseProtocolID() pb.ProtocolID
	GetProtocolRequestBasicData() *pb.BasicData
	GetProtocolResponseBasicData() *pb.BasicData
	GetProtocolResponseRetCode() *pb.RetCode
	SetProtocolRequestSign(signature []byte) error
	CallProtocolRequestCallback() (interface{}, error)
	CallProtocolResponseCallback() (interface{}, error)
	GetPubsubSource() PubsubSourceType
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
	OnSeekMailboxResponse(protoreflect.ProtoMessage) (interface{}, error)
	OnSendMsgRequest(protoreflect.ProtoMessage) (interface{}, error)
	OnSendMsgResponse(protoreflect.ProtoMessage) (interface{}, error)
	OnCustomPubsubProtocolRequest(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
	OnCustomPubsubProtocolResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
}

type ProtocolService interface {
	GetCurSrcUserPubKeyHex() string
	GetCurSrcUserSign(protoData []byte) ([]byte, error)
	PublishProtocol(protocolID pb.ProtocolID, userPubkey string, protocolData []byte, pubsubSource PubsubSourceType) error
}

type UserPubsub struct {
	Topic           *pubsub.Topic
	Subscription    *pubsub.Subscription
	IsReadPubsubMsg bool
	CancelFunc      context.CancelFunc
}
type SrcUserInfo struct {
	UserPubsub
	MailboxPeerId       string
	MailboxCreateSignal chan bool
	UserKey             *SrcUserKey
	GetSignCallback     GetSignCallback
}

type DestUserInfo struct {
	UserPubsub
}

type SrcUserKey struct {
	PubkeyHex string
	Pubkey    *ecdsa.PublicKey
}

type PubsubSourceType int32
type PubsubSourceStruct struct {
	DestUser PubsubSourceType
	SrcUser  PubsubSourceType
}

var PubsubSource = PubsubSourceStruct{
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

type GetSignCallback func(protoData []byte) (sig []byte, err error)

type CustomStreamProtocolInfo struct {
	Client   customProtocol.CustomStreamProtocolClient
	Protocol *StreamProtocol
}

type CustomPubsubProtocolInfo struct {
	Client   customProtocol.CustomPubsubProtocolClient
	Protocol *PubsubProtocol
}
