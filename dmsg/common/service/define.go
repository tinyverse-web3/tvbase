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
	"google.golang.org/protobuf/reflect/protoreflect"
)

type BaseService interface {
	IsEnableService() bool
	GetConfig() *config.DMsgConfig
	PublishProtocol(ctx context.Context, target *dmsgUser.Target, pid pb.PID, protoData []byte) error
}

type CommonService interface {
	BaseService
	GetUserPubkeyHex() (string, error)
	GetUserSig(protoData []byte) ([]byte, error)
	GetPublishTarget(request protoreflect.ProtoMessage) (*dmsgUser.Target, error)
	Start(
		enableService bool,
		pubkeyData []byte,
		getSig dmsgKey.GetSigCallback,
		timeout time.Duration,
	) error
	Stop() error
}

type MailboxService interface {
	CommonService
	SetOnMsgRequest(cb msg.OnMsgRequest)
	ReadMailbox(timeout time.Duration) ([]msg.Msg, error)
}

type MsgService interface {
	CommonService
	GetDestUser(pubkey string) *dmsgUser.ProxyPubsub
	SubscribeDestUser(pubkey string) error
	UnsubscribeDestUser(pubkey string) error
	SetOnMsgRequest(onMsgReceive msg.OnMsgRequest)
	SetOnMsgResponse(onMsgResponse msg.OnMsgResponse)
	SendMsg(destPubkey string, content []byte) (*pb.MsgReq, error)
}

type ChannelService interface {
	CommonService
	SubscribeChannel(pubkey string) error
	UnsubscribeChannel(pubkey string) error
	SetOnMsgRequest(onMsgRequest msg.OnMsgRequest)
	SetOnMsgResponse(onMsgResponse msg.OnMsgResponse)
	SendMsg(destPubkey string, content []byte) (*pb.MsgReq, error)
}

type CustomProtocolService interface {
	CommonService
	RegistClient(client customProtocol.ClientHandle) error
	UnregistClient(client customProtocol.ClientHandle) error
	RegistServer(service customProtocol.ServerHandle) error
	UnregistServer(callback customProtocol.ServerHandle) error
	Request(peerID string, pid string, content []byte) error
}
