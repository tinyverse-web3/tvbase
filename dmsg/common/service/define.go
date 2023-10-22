package service

import (
	"context"
	"time"

	"github.com/tinyverse-web3/tvbase/common/config"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	customProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol/custom"
)

var QueryPeerTopicName = "QueryPeer"

func SetQueryPeerTopicName(name string) {
	QueryPeerTopicName = name
}

func GetQueryPeerTopicName() string {
	return QueryPeerTopicName
}

type BaseService interface {
	GetConfig() *config.DMsgConfig
	PublishProtocol(ctx context.Context, target *dmsgUser.Target, pid pb.PID, protoData []byte) error
}

type CommonService interface {
	BaseService
	GetUserPubkeyHex() (string, error)
	GetUserSig(protoData []byte) ([]byte, error)
	GetPublishTarget(pubkey string) (*dmsgUser.Target, error)
	Stop() error
}

type MailboxService interface {
	CommonService
	Start(enableRequest bool, pubkey string, getSig dmsgKey.GetSigCallback) error
	CreateMailbox(pubkey string, timeout time.Duration) error
	CreateUserMailbox(timeout time.Duration) error
	CreateProxyMailbox(pubkey string, timeout time.Duration) error
	SetOnReceiveMsg(cb msg.OnReceiveMsg)
	ReadMailbox(timeout time.Duration) ([]msg.ReceiveMsg, error)
	TickReadMailbox(checkDuration time.Duration, readMailboxTimeout time.Duration)
}

type MsgService interface {
	CommonService
	IsExistDestUser(pubkey string) bool
	GetDestUser(pubkey string) *dmsgUser.ProxyPubsub
	SubscribeDestUser(pubkey string) error
	UnsubscribeDestUser(pubkey string) error
	SetOnReceiveMsg(onMsgReceive msg.OnReceiveMsg)
	SetOnRespondMsg(onMsgResponse msg.OnRespondMsg)
	SendMsg(destPubkey string, content []byte) (*pb.MsgReq, error)
	Start(
		enableService bool,
		pubkey string,
		getSig dmsgKey.GetSigCallback,
		isListenMsg bool,
	) error
}

type ChannelService interface {
	CommonService
	IsExistChannel(pubkey string) bool
	GetChannel(pubkey string) *dmsgUser.ProxyPubsub
	SubscribeChannel(pubkey string) error
	UnsubscribeChannel(pubkey string) error
	SetOnReceiveMsg(onMsgRequest msg.OnReceiveMsg)
	SetOnRespondMsg(onMsgResponse msg.OnRespondMsg)
	SendMsg(destPubkey string, content []byte) (*pb.MsgReq, error)
	Start(
		enableService bool,
		pubkey string,
		getSig dmsgKey.GetSigCallback,
	) error
}

type CustomProtocolService interface {
	CommonService
	RegistClient(client customProtocol.ClientHandle) error
	UnregistClient(client customProtocol.ClientHandle) error
	RegistServer(service customProtocol.ServerHandle) error
	UnregistServer(callback customProtocol.ServerHandle) error
	QueryPeer(pid string) (*pb.QueryPeerReq, chan any, error)
	Request(peerId string, pid string, content []byte) (*pb.CustomProtocolReq, chan any, error)
	Start(
		enableService bool,
		pubkey string,
		getSig dmsgKey.GetSigCallback,
	) error
}
