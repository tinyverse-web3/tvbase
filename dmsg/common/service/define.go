package service

import (
	"context"
	"time"

	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/define"
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
}

type MailboxService interface {
	CommonService
	Init(tvbaseService define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) error
	Release() error
	Start() error
	Stop() error
}

type MailboxClient interface {
	CommonService
	Init(tvbaseService define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) error
	Release() error
	CreateMailbox(timeout time.Duration) (existMailbox bool, err error)
	ReadMailbox(timeout time.Duration) ([]msg.ReceiveMsg, error)
}

type MsgService interface {
	CommonService
	Init(tvbaseService define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) error
	Release() error
	Start() error
	Stop() error
	IsExistDestUser(pubkey string) bool
	GetDestUser(pubkey string) *dmsgUser.ProxyPubsub
}

type MsgClient interface {
	CommonService
	Init(tvbaseService define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback, isListenMsg bool) error
	Release() error
	IsExistDestUser(pubkey string) bool
	GetDestUser(pubkey string) *dmsgUser.ProxyPubsub
	SubscribeDestUser(pubkey string, isListenMsg bool) error
	UnSubscribeDestUser(pubkey string) error
	SetOnReceiveMsg(onMsgReceive msg.OnReceiveMsg)
	SetOnRespondMsg(onMsgResponse msg.OnRespondMsg)
	SendMsg(destPubkey string, content []byte) (*pb.MsgReq, error)
}

type ChannelService interface {
	CommonService
	Init(tvbaseService define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) error
	Release() error
	Start() error
	Stop() error
	IsExistChannel(pubkey string) bool
	GetChannel(pubkey string) *dmsgUser.ProxyPubsub
}

type ChannelClient interface {
	CommonService
	Init(tvbaseService define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) error
	Release() error
	IsExistChannel(pubkey string) bool
	GetChannel(pubkey string) *dmsgUser.ProxyPubsub
	SubscribeChannel(pubkey string) error
	UnsubscribeChannel(pubkey string) error
	SetOnReceiveMsg(onMsgRequest msg.OnReceiveMsg)
	SetOnRespondMsg(onMsgResponse msg.OnRespondMsg)
	SendMsg(destPubkey string, content []byte) (*pb.MsgReq, error)
}
type CustomProtocolService interface {
	CommonService
	Init(tvbaseService define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) error
	Release() error
	Start() error
	Stop() error
	RegistServer(service customProtocol.ServerHandle) error
	UnregistServer(callback customProtocol.ServerHandle) error
}

type CustomProtocolClient interface {
	CommonService
	Init(tvbaseService define.TvBaseService, pubkey string, getSig dmsgKey.GetSigCallback) error
	Release() error
	RegistClient(client customProtocol.ClientHandle) error
	UnregistClient(client customProtocol.ClientHandle) error
	QueryPeer(pid string) (*pb.QueryPeerReq, chan any, error)
}
