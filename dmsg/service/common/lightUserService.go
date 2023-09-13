package common

import (
	"fmt"

	ipfsLog "github.com/ipfs/go-log/v2"
	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	tvutilKey "github.com/tinyverse-web3/tvutil/key"
)

var lightUserLog = ipfsLog.Logger("dmsg.service.lightuser")

type LightUserService struct {
	BaseService
	LightUser *dmsgUser.LightUser
}

func (d *LightUserService) Start(
	enableService bool,
	pubkeyData []byte,
	getSig dmsgKey.GetSigCallback,
	enableUserPubsub bool,
) error {
	d.BaseService.Start(enableService)
	return d.initUser(pubkeyData, getSig, enableUserPubsub)
}

func (d *LightUserService) Stop() error {
	err := d.unsubscribeUser()
	if err != nil {
		return err
	}
	return nil
}

// DmsgServiceInterface

func (d *LightUserService) GetUserPubkeyHex() (string, error) {
	if d.LightUser == nil {
		return "", fmt.Errorf("LightUserService->GetUserPubkeyHex: user is nil")
	}
	return d.LightUser.Key.PubkeyHex, nil
}

func (d *LightUserService) GetUserSig(protoData []byte) ([]byte, error) {
	if d.LightUser == nil {
		lightUserLog.Errorf("LightUserService->GetUserSig: user is nil")
		return nil, fmt.Errorf("LightUserService->GetUserSig: user is nil")
	}
	return d.LightUser.GetSig(protoData)
}

func (d *LightUserService) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	if d.LightUser == nil {
		lightUserLog.Errorf("LightUserService->GetPublishTarget: user is nil")
		return nil, fmt.Errorf("LightUserService->GetPublishTarget: user is nil")
	}
	return &d.LightUser.Target, nil
}

// user
func (d *LightUserService) initUser(
	pubkeyData []byte,
	getSig dmsgKey.GetSigCallback,
	enableUserPubsub bool,
) error {
	lightUserLog.Debug("LightUserService->InitUser begin")
	pubkey := tvutilKey.TranslateKeyProtoBufToString(pubkeyData)
	err := d.subscribeUser(pubkey, getSig, enableUserPubsub)
	if err != nil {
		return err
	}

	lightUserLog.Debug("LightUserService->InitUser end")
	return nil
}

func (d *LightUserService) subscribeUser(
	pubkey string,
	getSig dmsgKey.GetSigCallback,
	enableUserPubsub bool,
) error {
	lightUserLog.Debugf("LightUserService->subscribeUser begin\npubkey: %s", pubkey)
	if d.LightUser != nil {
		lightUserLog.Errorf("LightUserService->subscribeUser: user isn't nil")
		return fmt.Errorf("LightUserService->subscribeUser: user isn't nil")
	}

	target, err := dmsgUser.NewTarget(pubkey, getSig)
	if err != nil {
		lightUserLog.Errorf("LightUserService->subscribeUser: NewUser error: %v", err)
		return err
	}

	if enableUserPubsub {
		err = target.InitPubsub(pubkey)
		if err != nil {
			lightUserLog.Errorf("LightUserService->subscribeUser: InitPubsub error: %v", err)
			return err
		}
	}

	d.LightUser = &dmsgUser.LightUser{
		Target: *target,
	}

	lightUserLog.Debugf("LightUserService->subscribeUser end")
	return nil
}

func (d *LightUserService) unsubscribeUser() error {
	lightUserLog.Debugf("LightUserService->UnsubscribeUser begin")
	if d.LightUser == nil {
		lightUserLog.Warnf("LightUserService->UnsubscribeUser: user is nil")
		return nil
	}
	d.LightUser.Close()
	d.LightUser = nil
	lightUserLog.Debugf("LightUserService->UnsubscribeUser end")
	return nil
}
