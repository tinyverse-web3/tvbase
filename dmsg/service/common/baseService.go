package common

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"unsafe"

	ipfsLog "github.com/ipfs/go-log/v2"
	"github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/common/config"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	dmsgProtocol "github.com/tinyverse-web3/tvbase/dmsg/protocol"
)

var baseLog = ipfsLog.Logger("dmsg.service.base")

type ProtocolHandle struct {
	PID    pb.PID
	Data   []byte
	Handle dmsgProtocol.ProtocolHandle
}

type waitMessage struct {
	target          *dmsgUser.Target
	messageChanList []chan *ProtocolHandle
}

var waitMessageList map[string]*waitMessage
var protocolHandleList map[pb.PID]dmsgProtocol.ProtocolHandle

type BaseService struct {
	TvBase        common.TvBaseService
	EnableService bool
}

func (d *BaseService) Start(enableService bool) {
	d.EnableService = enableService
}

func (d *BaseService) Init(baseService common.TvBaseService) error {
	d.TvBase = baseService
	var err error
	if err != nil {
		baseLog.Errorf("Service.Init: pubsub.NewGossipSub error: %v", err)
		return err
	}
	waitMessageList = make(map[string]*waitMessage)
	protocolHandleList = make(map[pb.PID]dmsgProtocol.ProtocolHandle)
	return nil
}

func (d *BaseService) GetConfig() *config.DMsgConfig {
	return &d.TvBase.GetConfig().DMsg
}

func (d *BaseService) RegistPubsubProtocol(pid pb.PID, handle dmsgProtocol.ProtocolHandle) {
	protocolHandleList[pid] = handle
}

func (d *BaseService) UnregistPubsubProtocol(pid pb.PID) {
	delete(protocolHandleList, pid)
}

func (d *BaseService) WaitPubsubProtocolData(target *dmsgUser.Target) (pb.PID, []byte, dmsgProtocol.ProtocolHandle, error) {
	m, err := target.WaitMsg(d.TvBase.GetCtx())
	if err != nil {
		baseLog.Warnf("BaseService->handlePubsubProtocol: target.WaitMsg error: %+v", err)
		return -1, nil, nil, err
	}

	baseLog.Debugf("BaseService->handlePubsubProtocol:\ntopic: %s\nreceivedFrom: %+v", *m.Topic, m.ReceivedFrom)

	protocolID, protocolIDLen, err := checkProtocolData(m.Data)
	if err != nil {
		baseLog.Errorf("BaseService->handlePubsubProtocol: CheckPubsubData error: %v", err)
		return -1, nil, nil, nil
	}

	protocolData := m.Data[protocolIDLen:]
	protocolHandle := protocolHandleList[protocolID]

	if protocolHandle == nil {
		baseLog.Warnf("BaseService->handlePubsubProtocol: no protocolHandle for protocolID: %d", protocolID)
		return -1, nil, nil, nil
	}

	return protocolID, protocolData, protocolHandle, nil
}

// DmsgServiceInterface
func (d *BaseService) IsEnableService() bool {
	return d.EnableService
}

func (d *BaseService) PublishProtocol(ctx context.Context, target *dmsgUser.Target, pid pb.PID, protoData []byte) error {
	buf, err := dmsgProtocol.GenProtoData(pid, protoData)
	if err != nil {
		baseLog.Errorf("Service->PublishProtocol: GenProtoData error: %v", err)
		return err
	}

	if err := target.Publish(ctx, buf); err != nil {
		baseLog.Errorf("Service->PublishProtocol: target.Publish error: %v", err)
		return fmt.Errorf("Service->PublishProtocol: target.Publish error: %v", err)
	}
	return nil
}

func WaitMessage(ctx context.Context, pk string) (chan *ProtocolHandle, error) {
	target := dmsgUser.GetTarget(pk)
	if target == nil {
		baseLog.Errorf("CommonService->WaitMessage: target is nil")
		return nil, fmt.Errorf("CommonService->WaitMessage: target is nil")
	}

	if waitMessageList[pk] == nil {
		waitMessageList[pk] = &waitMessage{
			target: target,
		}
		go func() {
			for {
				m, err := target.WaitMsg(ctx)
				if err != nil {
					for _, c := range waitMessageList[pk].messageChanList {
						close(c)
					}
					delete(waitMessageList, pk)
					baseLog.Warnf("BaseService->WaitMessage: target.WaitMsg error: %+v", err)
					return
				}

				baseLog.Debugf("BaseService->WaitMessage:\ntopic: %s\nreceivedFrom: %+v", *m.Topic, m.ReceivedFrom)

				protocolID, protocolIDLen, err := checkProtocolData(m.Data)
				if err != nil {
					baseLog.Errorf("BaseService->WaitMessage: CheckPubsubData error: %v", err)
					return
				}

				protocolData := m.Data[protocolIDLen:]
				protocolHandle := protocolHandleList[protocolID]

				if protocolHandle == nil {
					baseLog.Warnf("BaseService->WaitMessage: no protocolHandle for protocolID: %d", protocolID)
				} else {
					for _, c := range waitMessageList[pk].messageChanList {
						c <- &ProtocolHandle{
							PID:    protocolID,
							Data:   protocolData,
							Handle: protocolHandle,
						}
					}
				}
			}
		}()
	}

	handleChan := make(chan *ProtocolHandle)
	waitMessageList[pk].messageChanList = append(waitMessageList[pk].messageChanList, handleChan)
	return handleChan, nil
}

func checkProtocolData(pubsubData []byte) (pb.PID, int, error) {
	var protocolID pb.PID
	protocolIDLen := int(unsafe.Sizeof(protocolID))
	err := binary.Read(bytes.NewReader(pubsubData[0:protocolIDLen]), binary.LittleEndian, &protocolID)
	if err != nil {
		baseLog.Errorf("CommonService->checkProtocolData: protocolID parse error: %v", err)
		return -1, 0, err
	}
	maxProtocolId := pb.PID(len(pb.PID_name) - 1)
	if protocolID > maxProtocolId {
		baseLog.Errorf("CommonService->checkProtocolData: protocolID(%d) > maxProtocolId(%d)", protocolID, maxProtocolId)
		return -1, 0, err
	}
	dataLen := len(pubsubData)
	if dataLen <= protocolIDLen {
		baseLog.Errorf("CommonService->checkProtocolData: dataLen(%d) <= protocolIDLen(%d)", dataLen, protocolIDLen)
		return protocolID, protocolIDLen, err
	}
	return protocolID, protocolIDLen, nil
}
