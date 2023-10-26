package mailbox

import (
	"fmt"

	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MailboxBase struct {
	dmsgServiceCommon.BaseService
	createMailboxProtocol *dmsgProtocol.MailboxSProtocol
	releaseMailboxPrtocol *dmsgProtocol.MailboxSProtocol
	readMailboxMsgPrtocol *dmsgProtocol.MailboxSProtocol
	seekMailboxProtocol   *dmsgProtocol.MailboxPProtocol
	pubsubMsgProtocol     *dmsgProtocol.PubsubMsgProtocol
	lightMailboxUser      *dmsgUser.LightMailboxUser
}

// MailboxSpCallback
func (d *MailboxBase) OnCreateMailboxRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	return nil, nil, false, nil
}

func (d *MailboxBase) OnCreateMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *MailboxBase) OnReleaseMailboxRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {

	return nil, nil, false, nil
}

func (d *MailboxBase) OnReleaseMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *MailboxBase) OnReadMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	return nil, nil, false, nil
}

func (d *MailboxBase) OnReadMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

// MailboxPpCallback
func (d *MailboxBase) OnSeekMailboxRequest(requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	return nil, nil, false, nil
}

func (d *MailboxBase) OnSeekMailboxResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *MailboxBase) OnPubsubMsgRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	return nil, nil, true, nil
}

func (d *MailboxBase) OnPubsubMsgResponse(requestProtoData protoreflect.ProtoMessage, responseProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *MailboxBase) GetUserPubkeyHex() (string, error) {
	if d.lightMailboxUser == nil {
		log.Errorf("MailboxBase->GetUserPubkeyHex: user is nil")
		return "", fmt.Errorf("MailboxBase->GetUserPubkeyHex: user is nil")
	}
	return d.lightMailboxUser.Key.PubkeyHex, nil
}

func (d *MailboxBase) GetUserSig(protoData []byte) ([]byte, error) {
	if d.lightMailboxUser == nil {
		log.Errorf("MailboxBase->GetUserSig: user is nil")
		return nil, fmt.Errorf("MailboxBase->GetUserSig: user is nil")
	}
	return d.lightMailboxUser.GetSig(protoData)
}

func (d *MailboxBase) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	var target *dmsgUser.Target
	if d.lightMailboxUser.Key.PubkeyHex == pubkey {
		target = &d.lightMailboxUser.Target
	}

	if target == nil {
		log.Errorf("MailboxBase->GetPublishTarget: target is nil")
		return nil, fmt.Errorf("MailboxBase->GetPublishTarget: target is nil")
	}

	topicName := target.Pubsub.Topic.String()
	log.Debugf("MailboxBase->GetPublishTarget: target's topic name: %s", topicName)
	return target, nil
}

func (d *MailboxBase) getMsgPrefix(pubkey string) string {
	return msg.MsgPrefix + pubkey
}
