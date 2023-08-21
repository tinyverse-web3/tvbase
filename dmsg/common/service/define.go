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

type BaseService interface {
	IsEnableService() bool
	GetConfig() *config.DMsgConfig
	PublishProtocol(ctx context.Context, target *dmsgUser.Target, pid pb.PID, protoData []byte) error
}

type LightService interface {
	BaseService
	GetUserPubkeyHex() (string, error)
	GetUserSig(protoData []byte) ([]byte, error)
	GetPublishTarget(pubkey string) (*dmsgUser.Target, error)
	Start(enableService bool, pubkeyData []byte, getSig dmsgKey.GetSigCallback) error
	Stop() error
}

type MailboxService interface {
	LightService
	SetOnReceiveMsg(cb msg.OnReceiveMsg)
	RequestReadMailbox(timeout time.Duration) ([]msg.Msg, error)
}

type MsgService interface {
	LightService
	GetDestUser(pubkey string) *dmsgUser.ProxyPubsub
	SubscribeDestUser(pubkey string) error
	UnsubscribeDestUser(pubkey string) error
	SetOnReceiveMsg(onReceiveMsg msg.OnReceiveMsg)
	SetOnSendMsgResponse(onSendMsgResponse msg.OnReceiveMsg)
	SendMsg(destPubkey string, content []byte) (*pb.MsgReq, error)
}

type ChannelService interface {
	LightService
	SubscribeChannel(pubkey string) error
	UnsubscribeChannel(pubkey string) error
	SetOnReceiveMsg(onReceiveMsg msg.OnReceiveMsg)
	SetOnSendMsgResponse(onSendMsgResponse msg.OnReceiveMsg)
	SendMsg(destPubkey string, content []byte) (*pb.MsgReq, error)
}

type CustomProtocolService interface {
	LightService
	RegistClient(client customProtocol.ClientHandle) error
	UnregistClient(client customProtocol.ClientHandle) error
	RegistServer(service customProtocol.ServerHandle) error
	UnregistServer(callback customProtocol.ServerHandle) error
	Request(peerID string, pid string, content []byte) error
}
