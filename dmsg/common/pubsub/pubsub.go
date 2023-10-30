package pubsub

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
)

type Topic struct {
	refCount int32
	pt       *pubsub.Topic
	sub      *pubsub.Subscription
}

type PubsubMgr struct {
	pubsub    *pubsub.PubSub
	topicList map[string]*Topic
}

var pubsubMgr = &PubsubMgr{}

func NewPubsubMgr(ctx context.Context, host host.Host, opts ...pubsub.Option) (*PubsubMgr, error) {
	pubsubMgr = &PubsubMgr{}
	err := pubsubMgr.init(ctx, host, opts...)
	if err != nil {
		return nil, err
	}
	return pubsubMgr, nil
}

func GetPubsubMgr() *PubsubMgr {
	return pubsubMgr
}

func (d *PubsubMgr) init(ctx context.Context, host host.Host, opts ...pubsub.Option) error {
	ret, err := pubsub.NewGossipSub(ctx, host, opts...)
	if err != nil {
		return err
	}
	d.pubsub = ret
	d.topicList = make(map[string]*Topic)
	return err
}

func (d *PubsubMgr) Subscribe(topicName string) (*pubsub.Topic, *pubsub.Subscription, error) {
	topic := d.topicList[topicName]
	if topic == nil {
		pt, err := d.pubsub.Join(topicName)
		if err != nil {
			return nil, nil, err
		}

		sub, err := pt.Subscribe()
		if err != nil {
			return nil, nil, err
		}
		topic = &Topic{
			pt:       pt,
			sub:      sub,
			refCount: 1,
		}
		d.topicList[topicName] = topic
	} else {
		topic.refCount++
	}

	return topic.pt, topic.sub, nil
}

func (d *PubsubMgr) Unsubscribe(topicName string) error {
	topic := d.topicList[topicName]
	if topic == nil {
		return fmt.Errorf("PubsubMgr->Cancel: topic %s is not exist", topicName)
	}
	sub := topic.sub
	if sub == nil {
		return fmt.Errorf("PubsubMgr->Cancel: subscription is not exist")
	}
	topic.refCount--
	if topic.refCount > 0 {
		return nil
	} else {
		sub.Cancel()
		err := topic.pt.Close()
		if err != nil {
			return err
		}
		delete(d.topicList, topicName)
	}
	return nil
}
