package user

import (
	"context"
	"fmt"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/tinyverse-web3/tvbase/dmsg/common/key"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvutil/crypto"
)

type Pubsub struct {
	Topic        *pubsub.Topic
	Subscription *pubsub.Subscription
}

func NewPubsub(p *pubsub.PubSub, pk string) (*Pubsub, error) {
	pubsub := &Pubsub{}
	err := pubsub.Init(p, pk)
	return pubsub, err
}

func (p *Pubsub) Init(ps *pubsub.PubSub, pk string) error {
	var err error
	p.Topic, err = ps.Join(pk)
	if err != nil {
		dmsgLog.Logger.Errorf("Pubsub->Init: Join error: %v", err)
		return err
	}

	p.Subscription, err = p.Topic.Subscribe()
	if err != nil {
		dmsgLog.Logger.Errorf("Pubsub->Init: Subscribe error: %v", err)
		return err
	}
	return nil
}

func (p *Pubsub) Publish(ctx context.Context, protoData []byte, opts ...pubsub.PubOpt) error {
	err := p.Topic.Publish(ctx, protoData, opts...)
	return err
}

func (p *Pubsub) Next(ctx context.Context) (*pubsub.Message, error) {
	return p.Subscription.Next(ctx)
}

func NewTarget(c context.Context, pk string, getSig key.GetSigCallback) (*Target, error) {
	user := &Target{}
	err := user.InitWithPubkey(c, pk, getSig)
	return user, err
}

type Target struct {
	Pubsub
	Key       key.Key
	Ctx       context.Context
	CancelCtx context.CancelFunc
}

func (t *Target) InitWithPubkey(c context.Context, pk string, getSig key.GetSigCallback) error {
	key := key.NewKey()
	err := key.InitKeyWithPubkeyHex(pk, getSig)
	if err != nil {
		dmsgLog.Logger.Errorf("User->InitWithPubkey: key.InitKeyWithPubkeyHex error: %v", err)
		return err
	}
	t.Key = *key

	ctx, cancelFunc := context.WithCancel(c)
	t.Ctx = ctx
	t.CancelCtx = cancelFunc
	return nil
}

func (t *Target) InitPubsub(p *pubsub.PubSub, pk string) error {
	pubsub, err := NewPubsub(p, pk)
	if err != nil {
		dmsgLog.Logger.Errorf("User->InitWithPubkey: NewPubsub error: %v", err)
		return err
	}
	t.Pubsub = *pubsub
	return nil
}

func (t *Target) GetSig(protoData []byte) ([]byte, error) {
	if t.Key.Prikey != nil {
		sig, err := crypto.SignDataByEcdsa(t.Key.Prikey, protoData)
		if err != nil {
			dmsgLog.Logger.Errorf("User->GetSig: %v", err)
			return sig, err
		}
		return sig, nil
	}
	if t.Key.GetSig == nil {
		return nil, fmt.Errorf("User->GetSig: Key.GetSig is nil")
	}
	return t.Key.GetSig(protoData)
}

func (s *Target) Publish(protoData []byte, opts ...pubsub.PubOpt) error {
	s.Pubsub.Publish(s.Ctx, protoData, opts...)
	return nil
}

func (s *Target) WaitMsg() (*pubsub.Message, error) {
	return s.Pubsub.Next(s.Ctx)
}

func (u *Target) Close() error {
	u.CancelCtx()
	u.Subscription.Cancel()
	err := u.Topic.Close()
	if err != nil {
		dmsgLog.Logger.Errorf("User->Close: Topic.Close error: %v", err)
	}
	return err
}

type DestTarget struct {
	Target
	LastReciveTimestamp int64
}

type Channel struct {
	DestTarget
}

type LightUser struct {
	Target
}

type LightMailboxUser struct {
	Target
	ServicePeerID string
}

type ServiceMailboxUser struct {
	DestTarget
	MsgRWMutex sync.RWMutex
}
