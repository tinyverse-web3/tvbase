package dmsg

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"unsafe"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
)

type DmsgService struct {
	BaseService tvCommon.TvBaseService
	Pubsub      *pubsub.PubSub

	PubsubProtocolResSubscribes map[pb.ProtocolID]protocol.ResSubscribe
	PubsubProtocolReqSubscribes map[pb.ProtocolID]protocol.ReqSubscribe
}

func (d *DmsgService) Init(nodeService tvCommon.TvBaseService) error {
	d.BaseService = nodeService

	var err error
	d.Pubsub, err = pubsub.NewGossipSub(d.BaseService.GetCtx(), d.BaseService.GetHost())
	if err != nil {
		return err
	}

	d.PubsubProtocolReqSubscribes = make(map[pb.ProtocolID]protocol.ReqSubscribe)
	d.PubsubProtocolResSubscribes = make(map[pb.ProtocolID]protocol.ResSubscribe)

	return nil
}

func (d *DmsgService) RegPubsubProtocolReqCallback(protocolID pb.ProtocolID, subscribe protocol.ReqSubscribe) error {
	d.PubsubProtocolReqSubscribes[protocolID] = subscribe
	return nil
}

func (d *DmsgService) RegPubsubProtocolResCallback(protocolID pb.ProtocolID, subscribe protocol.ResSubscribe) error {
	d.PubsubProtocolResSubscribes[protocolID] = subscribe
	return nil
}

func (d *DmsgService) Start() error {
	return nil
}

func (d *DmsgService) Stop() error {

	return nil
}

func (d *DmsgService) GetBasicToMsgPrefix(srcUserPubkey string, destUserPubkey string) string {
	return MsgPrefix + srcUserPubkey + MsgKeyDelimiter + destUserPubkey
}

func (d *DmsgService) GetBasicFromMsgPrefix(srcUserPubkey string, destUserPubkey string) string {
	return MsgPrefix + destUserPubkey + MsgKeyDelimiter + srcUserPubkey
}

func (d *DmsgService) GetFullToMsgPrefix(sendMsgReq *pb.SendMsgReq) string {
	basicPrefix := d.GetBasicToMsgPrefix(sendMsgReq.SrcPubkey, sendMsgReq.BasicData.DestPubkey)
	direction := MsgDirection.To
	return basicPrefix + MsgKeyDelimiter +
		direction + MsgKeyDelimiter +
		sendMsgReq.BasicData.Id + MsgKeyDelimiter +
		strconv.FormatInt(sendMsgReq.BasicData.Timestamp, 10)
}

func (d *DmsgService) GetFullFromMsgPrefix(sendMsgReq *pb.SendMsgReq) string {
	basicPrefix := d.GetBasicFromMsgPrefix(sendMsgReq.SrcPubkey, sendMsgReq.BasicData.DestPubkey)
	direction := MsgDirection.From
	return basicPrefix + MsgKeyDelimiter +
		direction + MsgKeyDelimiter +
		sendMsgReq.BasicData.Id + MsgKeyDelimiter +
		strconv.FormatInt(sendMsgReq.BasicData.Timestamp, 10)
}

func (d *DmsgService) CheckPubsubData(pubsubData []byte) (pb.ProtocolID, int, error) {
	var protocolID pb.ProtocolID
	protocolIDLen := int(unsafe.Sizeof(protocolID))
	err := binary.Read(bytes.NewReader(pubsubData[0:protocolIDLen]), binary.LittleEndian, &protocolID)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->CheckPubsubData: protocolID parse error: %v", err)
		return protocolID, protocolIDLen, err
	}
	maxProtocolId := pb.ProtocolID(len(pb.ProtocolID_name) - 1)
	if protocolID > maxProtocolId {
		dmsgLog.Logger.Errorf("DmsgService->CheckPubsubData: protocolID(%d) > maxProtocolId(%d)", protocolID, maxProtocolId)
		return protocolID, protocolIDLen, err
	}
	dataLen := len(pubsubData)
	if dataLen <= protocolIDLen {
		dmsgLog.Logger.Errorf("DmsgService->CheckPubsubData: dataLen(%d) <= protocolIDLen(%d)", dataLen, protocolIDLen)
		return protocolID, protocolIDLen, err
	}
	return protocolID, protocolIDLen, nil
}
