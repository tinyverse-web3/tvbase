package service

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unsafe"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
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
		log.Logger.Errorf("Service.Init: pubsub.NewGossipSub error: %v", err)
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

// DmsgServiceInterface
func (d *BaseService) IsEnableService() bool {
	return d.EnableService
}

func (d *BaseService) PublishProtocol(target *dmsgUser.Target, pid pb.PID, protoData []byte) error {
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
