package common

import (
	"context"
	"crypto/ecdsa"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ProtocolService interface {
	GetCurSrcUserPubKeyHex() string
	GetCurSrcUserSig(protoData []byte) ([]byte, error)
	RegPubsubProtocolResCallback(protocolID pb.PID, subscribe dmsgProtocol.ResSubscribe) error
	RegPubsubProtocolReqCallback(protocolID pb.PID, subscribe dmsgProtocol.ReqSubscribe) error
	PublishProtocol(protocolID pb.PID, userPubkey string, protocolData []byte) error
}

type StreamProtocolCallback interface {
	OnCreateMailboxRequest(protoreflect.ProtoMessage) (interface{}, error)
	OnCreateMailboxResponse(protoreflect.ProtoMessage) (interface{}, error)
	OnReadMailboxMsgRequest(protoreflect.ProtoMessage) (interface{}, error)
	OnReleaseMailboxRequest(protoreflect.ProtoMessage) (interface{}, error)
	OnCustomStreamProtocolRequest(protoreflect.ProtoMessage) (interface{}, error)
	OnCustomStreamProtocolResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
}

type StreamProtocolAdapter interface {
	InitResponse(basicData *pb.BasicData, data interface{}) error
	GetResponsePID() pb.PID
	GetStreamResponsePID() protocol.ID
	GetStreamRequestPID() protocol.ID
	GetRequestBasicData() *pb.BasicData
	GetResponseBasicData() *pb.BasicData
	SetProtocolResponseRet(code int32, result string)
	SetProtocolResponseFailRet(errMsg string)
	SetResponseSig(signature []byte) error
	CallRequestCallback() (interface{}, error)
	CallResponseCallback() (interface{}, error)
}
type StreamProtocol struct {
	Ctx              context.Context
	Host             host.Host
	ProtocolService  ProtocolService
	Callback         StreamProtocolCallback
	ProtocolRequest  protoreflect.ProtoMessage
	ProtocolResponse protoreflect.ProtoMessage
	Adapter          StreamProtocolAdapter
	stream           network.Stream
}

// pubsubProtocol
type PubsubProtocolAdapter interface {
	InitResponse(basicData *pb.BasicData, data interface{}) error
	GetRequestPID() pb.PID
	GetResponsePID() pb.PID
	GetRequestBasicData() *pb.BasicData
	GetResponseBasicData() *pb.BasicData
	GetResponseRetCode() *pb.RetCode
	SetResponseSig(signature []byte) error
	CallRequestCallback() (interface{}, error)
}

type PubsubProtocolCallback interface {
	OnSeekMailboxRequest(protoreflect.ProtoMessage) (interface{}, error)
	OnSendMsgRequest(protoreflect.ProtoMessage) (interface{}, error)
	OnCustomPubsubProtocolRequest(protoreflect.ProtoMessage) (interface{}, error)
	OnCustomPubsubProtocolResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
}

type PubsubProtocol struct {
	Host             host.Host
	ProtocolService  ProtocolService
	Callback         PubsubProtocolCallback
	ProtocolRequest  protoreflect.ProtoMessage
	ProtocolResponse protoreflect.ProtoMessage
	Adapter          PubsubProtocolAdapter
}

type CommonPubsub struct {
	Topic        *pubsub.Topic
	CancelFunc   context.CancelFunc
	Subscription *pubsub.Subscription
}

type DestUserPubsub struct {
	CommonPubsub
	MsgRWMutex          sync.RWMutex
	LastReciveTimestamp int64
}

type CustomProtocolPubsub struct {
	CommonPubsub
}

type CustomStreamProtocolInfo struct {
	Service  customProtocol.CustomStreamProtocolService
	Protocol *StreamProtocol
}

type CustomPubsubProtocolInfo struct {
	Service  customProtocol.CustomPubsubProtocolService
	Protocol *PubsubProtocol
}

const MailboxLimitErr = "mailbox is limited"
const MailboxAlreadyExistErr = "dest pubkey already exists"
const MailboxAlreadyExistCode = 1

type UserInfo struct {
	UserKey *UserKey
}
type UserKey struct {
	PubKeyHex string
	PriKeyHex string
	PubKey    *ecdsa.PublicKey
	PriKey    *ecdsa.PrivateKey
}
