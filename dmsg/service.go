package dmsg

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"unsafe"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol"
)

type DmsgService struct {
	BaseService tvCommon.TvBaseService
	Pubsub      *pubsub.PubSub

	PubsubProtocolResSubscribes map[pb.PID]protocol.ResSubscribe
	PubsubProtocolReqSubscribes map[pb.PID]protocol.ReqSubscribe
}

func (d *DmsgService) Init(nodeService tvCommon.TvBaseService) error {
	d.BaseService = nodeService

	var err error
	d.Pubsub, err = pubsub.NewGossipSub(d.BaseService.GetCtx(), d.BaseService.GetHost())
	if err != nil {
		dmsgLog.Logger.Errorf("Init: failed to create pubsub: %v", err)
		return err
	}

	d.PubsubProtocolReqSubscribes = make(map[pb.PID]protocol.ReqSubscribe)
	d.PubsubProtocolResSubscribes = make(map[pb.PID]protocol.ResSubscribe)
	return nil
}

func (d *DmsgService) HandleProtocolWithPubsub(target *dmsgUser.Target) {
	for {
		m, err := target.WaitMsg()
		if err != nil {
			dmsgLog.Logger.Warnf("dmsgService->HandleProtocolWithPubsub: subscription.Next happen err, %+v", err)
			return
		}

		dmsgLog.Logger.Debugf("dmsgService->HandleProtocolWithPubsub:\ntopic: %s\nreceivedFrom: %+v", *m.Topic, m.ReceivedFrom)

		protocolID, protocolIDLen, err := d.CheckPubsubData(m.Data)
		if err != nil {
			dmsgLog.Logger.Errorf("dmsgService->HandleProtocolWithPubsub: CheckPubsubData error %v", err)
			continue
		}
		contentData := m.Data[protocolIDLen:]
		reqSubscribe := d.PubsubProtocolReqSubscribes[protocolID]
		if reqSubscribe != nil {
			err = reqSubscribe.HandleRequestData(contentData)
			if err != nil {
				dmsgLog.Logger.Warnf("dmsgService->HandleProtocolWithPubsub: HandleRequestData error %v", err)
			}
			continue
		} else {
			dmsgLog.Logger.Warnf("dmsgService->HandleProtocolWithPubsub: no find protocolId(%d) for reqSubscribe", protocolID)
		}
		// no protocol for resSubScribe in service
		resSubScribe := d.PubsubProtocolResSubscribes[protocolID]
		if resSubScribe != nil {
			err = resSubScribe.HandleResponseData(contentData)
			if err != nil {
				dmsgLog.Logger.Warnf("dmsgService->HandleProtocolWithPubsub: HandleResponseData error: %v", err)
			}
			continue
		} else {
			dmsgLog.Logger.Warnf("dmsgService->HandleProtocolWithPubsub: no find protocolId(%d) for resSubscribe", protocolID)
		}
	}
}

func (d *DmsgService) RegPubsubProtocolReqCallback(pid pb.PID, subscribe protocol.ReqSubscribe) error {
	d.PubsubProtocolReqSubscribes[pid] = subscribe
	return nil
}

func (d *DmsgService) RegPubsubProtocolResCallback(pid pb.PID, subscribe protocol.ResSubscribe) error {
	d.PubsubProtocolResSubscribes[pid] = subscribe
	return nil
}

func (d *DmsgService) GetMsgPrefix(pubkey string) string {
	return MsgPrefix + pubkey
}

func (d *DmsgService) GetBasicFromMsgPrefix(srcUserPubkey string, destUserPubkey string) string {
	return MsgPrefix + destUserPubkey + MsgKeyDelimiter + srcUserPubkey
}

func (d *DmsgService) GetFullFromMsgPrefix(sendMsgReq *pb.SendMsgReq) string {
	basicPrefix := d.GetBasicFromMsgPrefix(sendMsgReq.BasicData.Pubkey, sendMsgReq.DestPubkey)
	direction := MsgDirection.From
	return basicPrefix + MsgKeyDelimiter +
		direction + MsgKeyDelimiter +
		sendMsgReq.BasicData.ID + MsgKeyDelimiter +
		strconv.FormatInt(sendMsgReq.BasicData.TS, 10)
}

func (d *DmsgService) CheckPubsubData(pubsubData []byte) (pb.PID, int, error) {
	var protocolID pb.PID
	protocolIDLen := int(unsafe.Sizeof(protocolID))
	err := binary.Read(bytes.NewReader(pubsubData[0:protocolIDLen]), binary.LittleEndian, &protocolID)
	if err != nil {
		dmsgLog.Logger.Errorf("DmsgService->CheckPubsubData: protocolID parse error: %v", err)
		return protocolID, protocolIDLen, err
	}
	maxProtocolId := pb.PID(len(pb.PID_name) - 1)
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
