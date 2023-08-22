package user

import (
	"context"
	"fmt"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/tinyverse-web3/tvbase/dmsg/common/key"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	dmsgCommonPubsub "github.com/tinyverse-web3/tvbase/dmsg/common/pubsub"
	"github.com/tinyverse-web3/tvutil/crypto"
)

var targetList map[string]*Target

type Pubsub struct {
	Topic        *pubsub.Topic
	Subscription *pubsub.Subscription
}

func NewPubsub(pk string) (*Pubsub, error) {
	pubsub := &Pubsub{}
	err := pubsub.Init(pk)
	return pubsub, err
}

func (p *Pubsub) Init(pk string) error {
	var err error

	mgr := dmsgCommonPubsub.GetPubsubMgr()

	p.Topic, p.Subscription, err = mgr.Subscribe(pk)
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

func NewTarget(pk string, getSig key.GetSigCallback) (*Target, error) {
	if targetList == nil {
		targetList = make(map[string]*Target)
	}
	if targetList[pk] != nil {
		dmsgLog.Logger.Debugf("User->NewTarget: target is already exist for pk :%s", pk)
		targetList[pk].RefCount++
		return targetList[pk], nil
	}
	target := &Target{
		RefCount: 1,
	}
	err := target.InitWithPubkey(pk, getSig)
	if err != nil {
		return nil, err
	}
	targetList[pk] = target
	return target, nil
}

func GetTarget(pk string) *Target {
	return targetList[pk]
}

type Target struct {
	Pubsub
	Key      key.Key
	RefCount int
}

func (t *Target) InitWithPubkey(pk string, getSig key.GetSigCallback) error {
	key := key.NewKey()
	err := key.InitKeyWithPubkeyHex(pk, getSig)
	if err != nil {
		dmsgLog.Logger.Errorf("User->InitWithPubkey: key.InitKeyWithPubkeyHex error: %v", err)
		return err
	}
	t.Key = *key
	return nil
}

func (t *Target) InitPubsub(pk string) error {
	pubsub, err := NewPubsub(pk)
	if err != nil {
		dmsgLog.Logger.Errorf("User->InitPubsub: NewPubsub error: %v", err)
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

func (t *Target) Publish(ctx context.Context, protoData []byte, opts ...pubsub.PubOpt) error {
	t.Pubsub.Publish(ctx, protoData, opts...)
	return nil
}

func (t *Target) WaitMsg(ctx context.Context) (*pubsub.Message, error) {
	return t.Pubsub.Next(ctx)
}

func (t *Target) Close() error {
	topicName := t.Subscription.Topic()
	err := dmsgCommonPubsub.GetPubsubMgr().Unsubscribe(topicName)
	if err != nil {
		return err
	}

	target := targetList[t.Key.PubkeyHex]
	if target == nil {
		return fmt.Errorf("Target->Close: target is nil for pk :%s", t.Key.PubkeyHex)
	}
	if target.Key.PrikeyHex != t.Key.PubkeyHex {
		return fmt.Errorf("Target->Close: target.Key.PrikeyHex != pk")
	}
	target.RefCount--
	if target.RefCount <= 0 {
		delete(targetList, t.Key.PubkeyHex)
	}
	return nil
}

type DestTarget struct {
	Target
	LastTimestamp int64
}

type ProxyPubsub struct {
	DestTarget
	AutoClean bool
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
