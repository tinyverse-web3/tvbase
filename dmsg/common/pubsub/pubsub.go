package pubsub

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
)

type Topic struct {
	pt      *pubsub.Topic
	subList map[*pubsub.Subscription]*pubsub.Subscription
}

type PubsubMgr struct {
	pubsub    *pubsub.PubSub
	topicList map[string]*Topic
}

var pubsubMgr = &PubsubMgr{}

func NewPubsubMgr(ctx context.Context, host host.Host) (*PubsubMgr, error) {
	pubsubMgr = &PubsubMgr{}
	err := pubsubMgr.init(ctx, host)
	if err != nil {
		return nil, err
	}
	return pubsubMgr, nil
}

func GetPubsubMgr() *PubsubMgr {
	return pubsubMgr
}

func (d *PubsubMgr) init(ctx context.Context, host host.Host) error {
	ret, err := pubsub.NewGossipSub(ctx, host)
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
		topic = &Topic{
			pt:      pt,
			subList: make(map[*pubsub.Subscription]*pubsub.Subscription),
		}
		d.topicList[topicName] = topic
	}

	sub, err := topic.pt.Subscribe()
	topic.subList[sub] = sub
	if err != nil {
		return topic.pt, nil, err
	}

	return topic.pt, sub, nil
}

func (d *PubsubMgr) Unsubscribe(topicName string, subscription *pubsub.Subscription) error {
	topic := d.topicList[topicName]
	if topic == nil {
		return fmt.Errorf("PubsubMgr->Cancel: topic %s is not exist", topicName)
	}
	sub := topic.subList[subscription]
	if sub == nil {
		return fmt.Errorf("PubsubMgr->Cancel: subscription is not exist")
	}

	sub.Cancel()
	delete(topic.subList, sub)

	if len(topic.subList) != 0 {
		return nil
	}

	err := topic.pt.Close()
	if err != nil {
		return err
	}

	delete(d.topicList, topicName)
	return nil
}
