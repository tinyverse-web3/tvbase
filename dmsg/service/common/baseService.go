package common

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unsafe"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/common/config"

	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
)

type BaseService struct {
	TvBase             common.TvBaseService
	Pubsub             *pubsub.PubSub
	EnableService      bool
	ProtocolHandleList map[pb.PID]dmsgProtocol.ProtocolHandle
}

func (d *BaseService) Start(enableService bool) {
	d.EnableService = enableService
}

func (d *BaseService) Init(baseService common.TvBaseService) error {
	d.TvBase = baseService
	var err error
	d.Pubsub, err = pubsub.NewGossipSub(d.TvBase.GetCtx(), d.TvBase.GetHost())
	if err != nil {
		log.Errorf("Service.Init: pubsub.NewGossipSub error: %v", err)
		return err
	}

	d.ProtocolHandleList = make(map[pb.PID]dmsgProtocol.ProtocolHandle)
	return nil
}

func (d *BaseService) GetConfig() *config.DMsgConfig {
	return &d.TvBase.GetConfig().DMsg
}

func (d *BaseService) RegistPubsubProtocol(pid pb.PID, handle dmsgProtocol.ProtocolHandle) {
	d.ProtocolHandleList[pid] = handle
}

func (d *BaseService) UnregistPubsubProtocol(pid pb.PID) {
	delete(d.ProtocolHandleList, pid)
}

func (d *BaseService) CheckProtocolData(pubsubData []byte) (pb.PID, int, error) {
	var protocolID pb.PID
	protocolIDLen := int(unsafe.Sizeof(protocolID))
	err := binary.Read(bytes.NewReader(pubsubData[0:protocolIDLen]), binary.LittleEndian, &protocolID)
	if err != nil {
		log.Errorf("CommonService->checkProtocolData: protocolID parse error: %v", err)
		return -1, 0, err
	}
	maxProtocolId := pb.PID(len(pb.PID_name) - 1)
	if protocolID > maxProtocolId {
		log.Errorf("CommonService->checkProtocolData: protocolID(%d) > maxProtocolId(%d)", protocolID, maxProtocolId)
		return -1, 0, err
	}
	dataLen := len(pubsubData)
	if dataLen <= protocolIDLen {
		log.Errorf("CommonService->checkProtocolData: dataLen(%d) <= protocolIDLen(%d)", dataLen, protocolIDLen)
		return protocolID, protocolIDLen, err
	}
	return protocolID, protocolIDLen, nil
}

func (d *BaseService) WaitPubsubProtocolData(target *dmsgUser.Target) (pb.PID, []byte, dmsgProtocol.ProtocolHandle, error) {
	m, err := target.WaitMsg()
	if err != nil {
		log.Warnf("BaseService->handlePubsubProtocol: target.WaitMsg error: %+v", err)
		return -1, nil, nil, err
	}

	log.Debugf("BaseService->handlePubsubProtocol:\ntopic: %s\nreceivedFrom: %+v", m.Topic, m.ReceivedFrom)

	protocolID, protocolIDLen, err := d.CheckProtocolData(m.Data)
	if err != nil {
		log.Errorf("BaseService->handlePubsubProtocol: CheckPubsubData error: %v", err)
		return -1, nil, nil, nil
	}

	protocolData := m.Data[protocolIDLen:]
	protocolHandle := d.ProtocolHandleList[protocolID]

	if protocolHandle == nil {
		log.Warnf("BaseService->handlePubsubProtocol: no protocolHandle for protocolID: %d", protocolID)
		return -1, nil, nil, nil
	}

	return protocolID, protocolData, protocolHandle, nil
}

// DmsgServiceInterface
func (d *BaseService) IsEnableService() bool {
	return d.EnableService
}

func (d *BaseService) PublishProtocol(target *dmsgUser.Target, pid pb.PID, protoData []byte) error {
	buf, err := dmsgProtocol.GenProtoData(pid, protoData)
	if err != nil {
		log.Errorf("Service->PublishProtocol: GenProtoData error: %v", err)
		return err
	}

	if err := target.Publish(buf); err != nil {
		log.Errorf("Service->PublishProtocol: target.Publish error: %v", err)
		return fmt.Errorf("Service->PublishProtocol: target.Publish error: %v", err)
	}
	return nil
}
