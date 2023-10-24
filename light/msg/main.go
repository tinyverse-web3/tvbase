package main

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tinyverse-web3/mtv_go_utils/crypto"
	"github.com/tinyverse-web3/mtv_go_utils/key"
	"github.com/tinyverse-web3/tvbase/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	"github.com/tinyverse-web3/tvbase/dmsg/common/service"
	"github.com/tinyverse-web3/tvbase/light"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

var tb *tvbase.TvBase
var dmsgService *light.DmsgService

func startDmsg(srcPubkey *ecdsa.PublicKey, srcPrikey *ecdsa.PrivateKey, tb *tvbase.TvBase) error {
	userPubkeyData, err := key.ECDSAPublicKeyToProtoBuf(srcPubkey)
	if err != nil {
		return err
	}

	getSig := func(protoData []byte) ([]byte, error) {
		sig, err := crypto.SignDataByEcdsa(srcPrikey, protoData)
		if err != nil {
			light.Logger.Errorf("initDmsg: sign error: %v", err)
		}
		return sig, nil
	}
	pubkey := key.TranslateKeyProtoBufToString(userPubkeyData)
	dmsgService, err = light.CreateDmsgService(tb, pubkey, getSig, true)
	if err != nil {
		return err
	}
	err = dmsgService.Start()
	if err != nil {
		return err
	}
	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init srcSeed, destSeed, rootPath from cmd params
	srcSeed, _, channelSeed, rootPath := light.ParseCmdParams()
	rootPath, err := util.GetRootPath(rootPath)
	if err != nil {
		light.Logger.Fatalf("tvnode->main: GetRootPath: %v", err)
	}
	cfg, err := light.InitConfig()
	if err != nil || cfg == nil {
		light.Logger.Fatalf("tvnode->main: InitConfig: %v", err)
	}

	if cfg.Identity.PrivKey == "" {
		err = cfg.GenPrivKey()
		if err != nil {
			light.Logger.Fatalf("tvnode->main: GenPrivKey: %v", err)
		}
	}

	if light.IsTestEnv {
		light.SetTestEnv(cfg)
	}

	err = light.InitLog()
	if err != nil {
		light.Logger.Fatalf("tvnode->main: InitLog: %v", err)
	}

	//dest
	light.Logger.Infof("tvnode->main: init dest user seed")
	destPrikey, destPubKey := light.GetSeedKey(srcSeed)

	// channel
	light.Logger.Infof("tvnode->main: init channel seed")
	_, channelPubKey := light.GetSeedKey(channelSeed)

	tb, err = tvbase.NewTvbase(ctx, cfg, rootPath)
	if err != nil {
		light.Logger.Fatalf("tvnode->main: NewTvbase error: %v", err)
	}
	tb.Start()
	err = tb.WaitRendezvous(30 * time.Second)
	if err != nil {
		light.Logger.Fatalf("tvnode->main: WaitRendezvous error: %v", err)
	}
	defer func() {
		err = tb.Stop()
		if err != nil {
			light.Logger.Errorf("tvnode->main: tb.Stop: %v", err)
		}
	}()

	light.Logger.Infof("tvnode->main: init src user seed")
	srcPrikey, srcPubkey := light.GetSeedKey(srcSeed)

	err = startDmsg(srcPubkey, srcPrikey, tb)
	if err != nil {
		light.Logger.Fatalf("tvnode->main: startDmsg error: %v", err)
		return
	}
	defer func() {
		err = dmsgService.Stop()
		if err != nil {
			light.Logger.Errorf("tvnode->main: dmsgService.Stop: %v", err)
		}
	}()

	// msgService
	msgService := initMsgService(srcPrikey, destPrikey)
	if err != nil {
		light.Logger.Fatalf("tvnode->main: initMsgService error: %v", err)
		return
	}
	destPubkeyStr, err := light.GetPubkey(destPubKey)
	if err != nil {
		light.Logger.Fatalf("tvnode->main: GetPubkey error: %v", err)
	}
	light.Logger.Debugf("tvnode->main: destPubkeyStr: %v", destPubkeyStr)
	err = msgService.SubscribeDestUser(destPubkeyStr, true)
	if err != nil {
		tb.SetTracerStatus(err)
		light.Logger.Fatalf("tvnode->main: SetTracerStatus error: %v", err)
	}

	initMailService(srcPrikey, destPrikey)

	// publish channelService channel
	channelService := initChannelService(srcPrikey, destPrikey)
	if err != nil {
		light.Logger.Fatalf("tvnode->main: initChannelService error: %v", err)
		return
	}
	channelPubkeyStr, err := light.GetPubkey(channelPubKey)
	if err != nil {
		light.Logger.Fatalf("tvnode->main: GetPubkey error: %v", err)
	}
	light.Logger.Debugf("tvnode->main: GetPubkey channelPubkeyStr: %v", channelPubkeyStr)
	err = channelService.SubscribeChannel(channelPubkeyStr)
	if err != nil {
		tb.SetTracerStatus(err)
		light.Logger.Fatalf("tvnode->main: GetPubkey channelService SubscribeChannel error: %v", err)
	}

	// send msg to dest user with read from stdin
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			sendContent, err := reader.ReadString('\n')
			if err != nil {
				light.Logger.Errorf("tvnode->main: read string error: %v", err)
				continue
			}
			sendContent = sendContent[:len(sendContent)-1]
			encrypedContent, err := crypto.EncryptWithPubkey(destPubKey, []byte(sendContent))
			if err != nil {
				light.Logger.Errorf("tvnode->main: encrypt error: %v", err)
				continue
			}

			pubkeyStr := destPubkeyStr
			// pubkeyStr := channelPubkeyStr
			// encrypedContent = []byte(sendContent)
			sendMsgReq, err := dmsgService.GetMsgClient().SendMsg(pubkeyStr, encrypedContent)
			if err != nil {
				light.Logger.Errorf("tvnode->main: send msg: error: %v", err)
			}
			light.Logger.Infof("tvnode->main: send msg done->\nsrcPubKey:%v\ndestPubkey:%v\nid:%s, protocolID:%v, timestamp:%v,\nmsg:%v",
				sendMsgReq.BasicData.Pubkey,
				sendMsgReq.DestPubkey,
				sendMsgReq.BasicData.ID,
				sendMsgReq.BasicData.PID,
				sendMsgReq.BasicData.TS,
				sendContent,
			)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		light.Logger.Infof("tvnodelight->main: received interrupt signal: %v", sig)
		cancel()
	}()

	<-ctx.Done()
	light.Logger.Info("tvnodelight->main: gracefully shut down")
}

func initMsgService(srcPrikey *ecdsa.PrivateKey, destPrikey *ecdsa.PrivateKey) service.MsgClient {
	msgOnRequest := func(message *msg.ReceiveMsg) ([]byte, error) {
		var err error
		decrypedContent := []byte("")
		switch message.Direction {
		case msg.MsgDirection.To:
			decrypedContent, err = crypto.DecryptWithPrikey(destPrikey, message.Content)
			if err != nil {
				decrypedContent = []byte(err.Error())
				light.Logger.Errorf("decrypt error: %v", err)
			}
		case msg.MsgDirection.From:
			decrypedContent, err = crypto.DecryptWithPrikey(srcPrikey, message.Content)
			if err != nil {
				decrypedContent = []byte(err.Error())
				light.Logger.Errorf("decrypt error: %v", err)
			}
		}
		light.Logger.Infof("msgOnRequest-> \nsrcUserPubkey: %s, \ndestUserPubkey: %s, \nmsgContent: %s, time:%v, direction: %s",
			message.ReqPubkey, message.DestPubkey, string(decrypedContent), time.Unix(message.TimeStamp, 0), message.Direction)
		return nil, nil
	}

	msgOnResponse := func(message *msg.RespondMsg) {
		light.Logger.Infof("OnMsgResponse-> \nmessage: %+v", message)
	}

	ret := dmsgService.GetMsgClient()
	ret.SetOnReceiveMsg(msgOnRequest)
	ret.SetOnRespondMsg(msgOnResponse)
	return ret
}

func initChannelService(srcPrikey *ecdsa.PrivateKey, destPrikey *ecdsa.PrivateKey) service.ChannelClient {
	ret := dmsgService.GetChannelClient()
	channelOnRequest := func(message *msg.ReceiveMsg) ([]byte, error) {
		light.Logger.Infof("OnMsgResponse-> \nmessage: %+v", message)
		return nil, nil
	}
	channelOnResponse := func(message *msg.RespondMsg) {
		light.Logger.Infof("OnMsgResponse-> \nmessage: %+v", message)
	}
	ret.SetOnReceiveMsg(channelOnRequest)
	ret.SetOnRespondMsg(channelOnResponse)
	return ret
}

func initMailService(srcPrikey *ecdsa.PrivateKey, destPrikey *ecdsa.PrivateKey) {
	msgList, err := dmsgService.GetMailboxClient().ReadMailbox(3 * time.Second)
	if err != nil {
		light.Logger.Errorf("initMailService: read mailbox error: %v", err)
	}
	light.Logger.Debugf("initMailService: found (%d) new message", len(msgList))
}
