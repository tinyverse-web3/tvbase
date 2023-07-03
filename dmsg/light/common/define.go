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

type StreamProtocolCallback interface {
	OnCreateMailboxResponse(protoreflect.ProtoMessage) (interface{}, error)
	OnReadMailboxMsgResponse(protoreflect.ProtoMessage) (interface{}, error)
	OnReleaseMailboxResponse(protoreflect.ProtoMessage) (interface{}, error)
	OnSeekMailboxResponse(protoreflect.ProtoMessage) (interface{}, error)
	OnCustomProtocolResponse(protoreflect.ProtoMessage, protoreflect.ProtoMessage) (interface{}, error)
}
type StreamProtocolAdapter interface {
	InitProtocolRequest(basicData *pb.BasicData)
	SetCustomContent(customProtocolId string, requestContent []byte) error
	GetRequestProtocolID() pb.ProtocolID
	GetStreamRequestProtocolID() protocol.ID
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
	ProtocolRequest  protoreflect.ProtoMessage
	ProtocolResponse protoreflect.ProtoMessage
	Adapter          StreamProtocolAdapter
}

type PubsubProtocolAdapter interface {
	InitProtocolRequest(basicData *pb.BasicData)
	GetRequestProtocolID() pb.ProtocolID
	GetProtocolResponseBasicData() *pb.BasicData
	GetProtocolResponseRetCode() *pb.RetCode
	SetProtocolRequestSign() error
	CallProtocolResponseCallback() (interface{}, error)
	GetPubsubSource() PubsubSourceType
}

type PubsubProtocol struct {
	Ctx              context.Context
	Host             host.Host
	RequestInfoList  map[string]*RequestInfo
	Callback         PubsubProtocolCallback
	LightService     LightService
	ProtocolRequest  protoreflect.ProtoMessage
	ProtocolResponse protoreflect.ProtoMessage
	Adapter          PubsubProtocolAdapter
}

type PubsubProtocolCallback interface {
	OnSeekMailboxResponse(protoreflect.ProtoMessage) (interface{}, error)
	OnSendMsgResquest(protoreflect.ProtoMessage, []byte) (interface{}, error)
}

type LightService interface {
	RegPubsubProtocolResCallback(protocolID pb.ProtocolID, subscribe dmsgProtocol.ResSubscribe) error
	RegPubsubProtocolReqCallback(protocolID pb.ProtocolID, subscribe dmsgProtocol.ReqSubscribe) error
	PublishProtocol(protocolID pb.ProtocolID, userPubkey string, protocolData []byte, pubsubSource PubsubSourceType) error
	SaveUserMsg(protoMsg protoreflect.ProtoMessage, msgDirection string) error
}

type UserPubsub struct {
	UserTopic *pubsub.Topic
	UserSub   *pubsub.Subscription
}
type SrcUserInfo struct {
	UserPubsub
	MailboxPeerId       string
	MailboxCreateSignal chan bool
	MsgRWMutex          sync.RWMutex
	UserKey             *SrcUserKey
}

type DestUserInfo struct {
	UserPubsub
}

type SrcUserKey struct {
	PubkeyHex string
	PrikeyHex string
	Prikey    *ecdsa.PrivateKey
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

type UserMsgByTimeStamp []UserMsg

type CustomProtocolInfo struct {
	Light          customProtocol.CustomProtocolLight
	StreamProtocol *StreamProtocol
}

func (a UserMsgByTimeStamp) Len() int           { return len(a) }
func (a UserMsgByTimeStamp) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a UserMsgByTimeStamp) Less(i, j int) bool { return a[i].TimeStamp < a[j].TimeStamp }
