package dmsg

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"unsafe"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"

	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
)

type Service struct {
	baseService        common.TvBaseService
	pubsub             *pubsub.PubSub
	protocolHandleList map[pb.PID]dmsgProtocol.ProtocolHandle
	enableService      bool
}

func (d *Service) Start(enableService bool) error {
	d.enableService = enableService
	return nil
}

func (d *Service) Stop() error {
	return nil
}

func (d *Service) Init(baseService common.TvBaseService) error {
	d.baseService = baseService
	var err error
	d.pubsub, err = pubsub.NewGossipSub(d.baseService.GetCtx(), d.baseService.GetHost())
	if err != nil {
		log.Logger.Errorf("Service.Init: pubsub.NewGossipSub error: %v", err)
		return err
	}

	d.protocolHandleList = make(map[pb.PID]dmsgProtocol.ProtocolHandle)
	return nil
}

func (d *Service) GetConfig() *config.DMsgConfig {
	return &d.baseService.GetConfig().DMsg
}

func (d *Service) IsEnableService() bool {
	return d.enableService
}

func (d *Service) registPubsubProtocol(pid pb.PID, handle dmsgProtocol.ProtocolHandle) {
	d.protocolHandleList[pid] = handle
}

func (d *Service) unregistPubsubProtocol(pid pb.PID) {
	delete(d.protocolHandleList, pid)
}

func (d *Service) checkProtocolData(pubsubData []byte) (pb.PID, int, error) {
	var protocolID pb.PID
	protocolIDLen := int(unsafe.Sizeof(protocolID))
	err := binary.Read(bytes.NewReader(pubsubData[0:protocolIDLen]), binary.LittleEndian, &protocolID)
	if err != nil {
		log.Logger.Errorf("CommonService->checkProtocolData: protocolID parse error: %v", err)
		return protocolID, protocolIDLen, err
	}
	maxProtocolId := pb.PID(len(pb.PID_name) - 1)
	if protocolID > maxProtocolId {
		log.Logger.Errorf("CommonService->checkProtocolData: protocolID(%d) > maxProtocolId(%d)", protocolID, maxProtocolId)
		return protocolID, protocolIDLen, err
	}
	dataLen := len(pubsubData)
	if dataLen <= protocolIDLen {
		log.Logger.Errorf("CommonService->checkProtocolData: dataLen(%d) <= protocolIDLen(%d)", dataLen, protocolIDLen)
		return protocolID, protocolIDLen, err
	}
	return protocolID, protocolIDLen, nil
}

func (d *Service) PublishProtocol(target *dmsgUser.Target, pid pb.PID, protoData []byte) error {
	buf, err := dmsgProtocol.GenProtoData(pid, protoData)
	if err != nil {
		log.Logger.Errorf("Service->PublishProtocol: GenProtoData error: %v", err)
		return err
	}

	if err := target.Publish(buf); err != nil {
		log.Logger.Errorf("Service->PublishProtocol: target.Publish error: %v", err)
		return fmt.Errorf("Service->PublishProtocol: target.Publish error: %v", err)
	}
	return nil
}

func (d *Service) getMsgPrefix(pubkey string) string {
	return msg.MsgPrefix + pubkey
}

func (d *Service) getBasicFromMsgPrefix(srcUserPubkey string, destUserPubkey string) string {
	return msg.MsgPrefix + destUserPubkey + msg.MsgKeyDelimiter + srcUserPubkey
}

func (d *Service) getFullFromMsgPrefix(sendMsgReq *pb.SendMsgReq) string {
	basicPrefix := d.getBasicFromMsgPrefix(sendMsgReq.BasicData.Pubkey, sendMsgReq.DestPubkey)
	direction := msg.MsgDirection.From
	return basicPrefix + msg.MsgKeyDelimiter +
		direction + msg.MsgKeyDelimiter +
		sendMsgReq.BasicData.ID + msg.MsgKeyDelimiter +
		strconv.FormatInt(sendMsgReq.BasicData.TS, 10)
}
