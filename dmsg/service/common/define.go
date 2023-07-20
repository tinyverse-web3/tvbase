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
	GetCurSrcUserSign(protoData []byte) ([]byte, error)
	RegPubsubProtocolResCallback(protocolID pb.ProtocolID, subscribe dmsgProtocol.ResSubscribe) error
	RegPubsubProtocolReqCallback(protocolID pb.ProtocolID, subscribe dmsgProtocol.ReqSubscribe) error
	PublishProtocol(protocolID pb.ProtocolID, userPubkey string, protocolData []byte) error
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
	InitProtocolResponse(basicData *pb.BasicData, data interface{}) error
	GetResponseProtocolID() pb.ProtocolID
	GetStreamResponseProtocolID() protocol.ID
	GetStreamRequestProtocolID() protocol.ID
	GetProtocolRequestBasicData() *pb.BasicData
	GetProtocolResponseBasicData() *pb.BasicData
	SetProtocolResponseRet(code int32, result string)
	SetProtocolResponseFailRet(errMsg string)
	SetProtocolResponseSign(signature []byte) error
	CallProtocolRequestCallback() (interface{}, error)
	CallProtocolResponseCallback() (interface{}, error)
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
	InitProtocolResponse(basicData *pb.BasicData, data interface{}) error
	GetRequestProtocolID() pb.ProtocolID
	GetResponseProtocolID() pb.ProtocolID
	GetProtocolRequestBasicData() *pb.BasicData
	GetProtocolResponseBasicData() *pb.BasicData
	GetProtocolResponseRetCode() *pb.RetCode
	SetProtocolResponseSign(signature []byte) error
	CallProtocolRequestCallback() (interface{}, error)
}

type PubsubProtocolCallback interface {
	OnSeekMailboxRequest(protoreflect.ProtoMessage) (interface{}, error)
	OnCustomPubsubProtocolRequest(protoreflect.ProtoMessage) (interface{}, error)
	OnCustomPubsubProtocolResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
	OnHandleSendMsgRequest(protoreflect.ProtoMessage, []byte) (interface{}, error)
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
