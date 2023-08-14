package common

import (
	"fmt"

	dmsgKey "github.com/tinyverse-web3/tvbase/dmsg/common/key"
	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
	dmsgUser "github.com/tinyverse-web3/tvbase/dmsg/common/user"
	tvutilKey "github.com/tinyverse-web3/tvutil/key"
)

type LightUserService struct {
	BaseService
	LightUser *dmsgUser.LightUser
}

func (d *LightUserService) Start(
	enableService bool,
	pubkeyData []byte,
	getSig dmsgKey.GetSigCallback,
	subscribeUser bool,
) error {
	d.BaseService.Start(enableService)
	return d.InitUser(pubkeyData, getSig, subscribeUser)
}

func (d *LightUserService) Stop() error {
	err := d.UnsubscribeUser()
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
		log.Logger.Errorf("LightUserService->GetUserSig: user is nil")
		return nil, fmt.Errorf("LightUserService->GetUserSig: user is nil")
	}
	return d.LightUser.GetSig(protoData)
}

func (d *LightUserService) GetPublishTarget(pubkey string) (*dmsgUser.Target, error) {
	if d.LightUser == nil {
		log.Logger.Errorf("LightUserService->GetPublishTarget: user is nil")
		return nil, fmt.Errorf("LightUserService->GetPublishTarget: user is nil")
	}
	return &d.LightUser.Target, nil
}

// user
func (d *LightUserService) InitUser(pubkeyData []byte, getSig dmsgKey.GetSigCallback, subscribeUser bool) error {
	log.Logger.Debug("LightUserService->InitUser begin")
	pubkey := tvutilKey.TranslateKeyProtoBufToString(pubkeyData)
	err := d.SubscribeUser(pubkey, getSig, subscribeUser)
	if err != nil {
		return err
	}

	log.Logger.Debug("LightUserService->InitUser end")
	return nil
}

func (d *LightUserService) SubscribeUser(pubkey string, getSig dmsgKey.GetSigCallback, subscribeUser bool) error {
	log.Logger.Debugf("LightUserService->SubscribeUser begin\npubkey: %s", pubkey)
	if d.LightUser != nil {
		log.Logger.Errorf("LightUserService->SubscribeUser: user isn't nil")
		return fmt.Errorf("LightUserService->SubscribeUser: user isn't nil")
	}

	target, err := dmsgUser.NewTarget(d.TvBase.GetCtx(), pubkey, getSig)
	if err != nil {
		log.Logger.Errorf("LightUserService->SubscribeUser: NewUser error: %v", err)
		return err
	}

	if subscribeUser {
		err = target.InitPubsub(d.Pubsub, pubkey)
		if err != nil {
			log.Logger.Errorf("LightUserService->SubscribeUser: InitPubsub error: %v", err)
			return err
		}
	}

	d.LightUser = &dmsgUser.LightUser{
		Target: *target,
	}

	log.Logger.Debugf("LightUserService->SubscribeUser end")
	return nil
}

func (d *LightUserService) UnsubscribeUser() error {
	log.Logger.Debugf("LightUserService->UnsubscribeUser begin")
	if d.LightUser == nil {
		log.Logger.Warnf("LightUserService->UnsubscribeUser: user is nil")
		return nil
	}
	err := d.LightUser.Close()
	if err != nil {
		log.Logger.Warnf("LightUserService->UnsubscribeUser: Close error: %v", err)
	}
	d.LightUser = nil
	log.Logger.Debugf("LightUserService->UnsubscribeUser end")
	return nil
}
