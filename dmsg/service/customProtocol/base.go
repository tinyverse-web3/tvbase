package customProtocol

import (
	"fmt"

	tvutilKey "github.com/tinyverse-web3/mtv_go_utils/key"
	dmsgCommonService "github.com/tinyverse-web3/tvbase/dmsg/common/service"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	dmsgServiceCommon "github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type CustomProtocolBase struct {
	dmsgServiceCommon.LightUserService
	queryPeerTarget *dmsgUser.Target
}

// sdk-common

func (d *CustomProtocolBase) OnCustomRequest(
	requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	return nil, nil, false, nil
}

func (d *CustomProtocolBase) OnCustomResponse(
	requestProtoData protoreflect.ProtoMessage,
	responseProtoData protoreflect.ProtoMessage) (any, error) {
	return nil, nil
}

func (d *CustomProtocolBase) OnQueryPeerRequest(requestProtoData protoreflect.ProtoMessage) (any, any, bool, error) {
	return "", nil, false, nil
}

func (d *CustomProtocolBase) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	if d.queryPeerTarget == nil {
		log.Errorf("CustomProtocolBase->GetPublishTarget: queryPeerTarget is nil")
		return nil, fmt.Errorf("CustomProtocolBase->GetPublishTarget: queryPeerTarget is nil")
	}
	return d.queryPeerTarget, nil
}

func (d *CustomProtocolBase) GetQueryPeerPubkey() string {
	topicName := dmsgCommonService.GetQueryPeerTopicName()
	_, topicNamePubkey, err := tvutilKey.GenerateEcdsaKey(topicName)
	if err != nil {
		return ""
	}
	topicNamePubkeyData, err := tvutilKey.ECDSAPublicKeyToProtoBuf(topicNamePubkey)
	if err != nil {
		log.Errorf("CustomProtocolService->GetQueryPeerPubkey: ECDSAPublicKeyToProtoBuf error: %v", err)
		return ""
	}
	topicNamePubkeyHex := tvutilKey.TranslateKeyProtoBufToString(topicNamePubkeyData)
	return topicNamePubkeyHex
}
