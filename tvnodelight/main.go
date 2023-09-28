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
	"github.com/tinyverse-web3/tvbase/tvbase"
)

var tb *tvbase.TvBase

func startDmsg(srcPubkey *ecdsa.PublicKey, srcPrikey *ecdsa.PrivateKey, tb *tvbase.TvBase) error {
	userPubkeyData, err := key.ECDSAPublicKeyToProtoBuf(srcPubkey)
	if err != nil {
		logger.Errorf("initDmsg: ECDSAPublicKeyToProtoBuf error: %v", err)
		return err
	}

	getSig := func(protoData []byte) ([]byte, error) {
		sig, err := crypto.SignDataByEcdsa(srcPrikey, protoData)
		if err != nil {
			logger.Errorf("initDmsg: sign error: %v", err)
		}
		return sig, nil
	}

	err = tb.GetDmsgService().Start(false, userPubkeyData, getSig, 30*time.Second)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init srcSeed, destSeed, rootPath from cmd params
	srcSeed, _, channelSeed, rootPath := parseCmdParams()
	rootPath, err := util.GetRootPath(rootPath)
	if err != nil {
		logger.Fatalf("tvnode->main: GetRootPath: %v", err)
	}
	cfg, err := loadConfig(rootPath)
	if err != nil || cfg == nil {
		logger.Fatalf("tvnode->main: loadConfig: %v", err)
	}

	err = initLog()
	if err != nil {
		logger.Fatalf("tvnode->main: initLog: %v", err)
	}

	//dest
	logger.Infof("tvnode->main: init dest user seed")
	destPrikey, destPubKey := getSeedKey(srcSeed)

	// channel
	logger.Infof("tvnode->main: init channel seed")
	_, channelPubKey := getSeedKey(channelSeed)

	tb, err := tvbase.NewTvbase(ctx, cfg, rootPath)
	if err != nil {
		logger.Fatalf("NewTvbase error: %v", err)
	}
	tb.Start()
	dmsgService := tb.GetDmsgService()

	logger.Infof("tvnode->main: init src user seed")
	srcPrikey, srcPubkey := getSeedKey(srcSeed)

	err = startDmsg(srcPubkey, srcPrikey, tb)
	if err != nil {
		logger.Fatalf("startDmsgService error: %v", err)
		return
	}

	// msgService
	msgService := initMsgService(srcPrikey, destPrikey)
	if err != nil {
		logger.Fatalf("initMsgService error: %v", err)
		return
	}
	destPubkeyStr, err := getPubkey(destPubKey)
	if err != nil {
		logger.Fatalf("msgService getPubkey error: %v", err)
	}
	logger.Debugf("destPubkeyStr: %v", destPubkeyStr)
	err = msgService.SubscribeDestUser(destPubkeyStr)
	if err != nil {
		tb.SetTracerStatus(err)
		logger.Fatalf("SubscribeDestUser error: %v", err)
	}

	initMailService(srcPrikey, destPrikey)

	// publish channelService channel
	channelService := initChannelService(srcPrikey, destPrikey)
	if err != nil {
		logger.Fatalf("initChannelService error: %v", err)
		return
	}
	channelPubkeyStr, err := getPubkey(channelPubKey)
	if err != nil {
		logger.Fatalf("channelService getPubkey error: %v", err)
	}
	logger.Debugf("channelPubkeyStr: %v", channelPubkeyStr)
	err = channelService.SubscribeChannel(channelPubkeyStr)
	if err != nil {
		tb.SetTracerStatus(err)
		logger.Fatalf("channelService SubscribeChannel error: %v", err)
	}

	// send msg to dest user with read from stdin
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			sendContent, err := reader.ReadString('\n')
			if err != nil {
				logger.Errorf("read string error: %v", err)
				continue
			}
			sendContent = sendContent[:len(sendContent)-1]
			encrypedContent, err := crypto.EncryptWithPubkey(destPubKey, []byte(sendContent))
			if err != nil {
				logger.Errorf("encrypt error: %v", err)
				continue
			}

			pubkeyStr := destPubkeyStr
			// pubkeyStr := channelPubkeyStr
			// encrypedContent = []byte(sendContent)
			sendMsgReq, err := dmsgService.GetMsgService().SendMsg(pubkeyStr, encrypedContent)
			if err != nil {
				logger.Errorf("send msg: error: %v", err)
			}
			logger.Infof("send msg done->\nsrcPubKey:%v\ndestPubkey:%v\nid:%s, protocolID:%v, timestamp:%v,\nmsg:%v",
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
		logger.Infof("tvnodelight->main: received interrupt signal: %v", sig)
		cancel()
	}()

	<-ctx.Done()
	logger.Info("tvnodelight->main: gracefully shut down")
}

func initMsgService(srcPrikey *ecdsa.PrivateKey, destPrikey *ecdsa.PrivateKey) service.MsgService {
	msgOnRequest := func(srcPubkey string, destPubkey string, msgContent []byte, timeStamp int64, msgID string, direction string) ([]byte, error) {
		var err error
		decrypedContent := []byte("")
		switch direction {
		case msg.MsgDirection.To:
			decrypedContent, err = crypto.DecryptWithPrikey(destPrikey, msgContent)
			if err != nil {
				decrypedContent = []byte(err.Error())
				logger.Errorf("decrypt error: %v", err)
			}
		case msg.MsgDirection.From:
			decrypedContent, err = crypto.DecryptWithPrikey(srcPrikey, msgContent)
			if err != nil {
				decrypedContent = []byte(err.Error())
				logger.Errorf("decrypt error: %v", err)
			}
		}
		logger.Infof("msgOnRequest-> \nsrcUserPubkey: %s, \ndestUserPubkey: %s, \nmsgContent: %s, time:%v, direction: %s",
			srcPubkey, destPubkey, string(decrypedContent), time.Unix(timeStamp, 0), direction)
		return nil, nil
	}

	msgOnResponse := func(requestPubkey string, requestDestPubkey string, responseDestPubkey string, responseContent []byte, timeStamp int64, msgID string,
	) ([]byte, error) {
		logger.Infof("OnMsgResponse-> \nrequestPubkey: %s, \nrequestDestPubkey: %s, \nresponseDestPubkey: %s, \nresponseContent: %s, time:%v, msgID: %s",
			requestPubkey, requestDestPubkey, responseDestPubkey, string(responseContent), time.Unix(timeStamp, 0), msgID)
		return nil, nil
	}

	ret := tb.GetDmsgService().GetMsgService()
	ret.SetOnMsgRequest(msgOnRequest)
	ret.SetOnMsgResponse(msgOnResponse)
	return ret
}

func initChannelService(srcPrikey *ecdsa.PrivateKey, destPrikey *ecdsa.PrivateKey) service.ChannelService {
	ret := tb.GetDmsgService().GetChannelService()
	channelOnRequest := func(requestPubkey string, requestDestPubkey string, requestContent []byte, timeStamp int64, msgID string, direction string) ([]byte, error) {
		logger.Infof("channelOnRequest-> \nrequestPubkey: %s, \nrequestDestPubkey: %s, \nrequestContent: %s, time:%v, direction: %s\nmsgId: %s",
			requestPubkey, requestDestPubkey, string(requestContent), time.Unix(timeStamp, 0), direction, msgID)
		return nil, nil
	}
	channelOnResponse := func(requestPubkey string, requestDestPubkey string, responsePubkey string, responseContent []byte, timeStamp int64, msgID string) ([]byte, error) {
		logger.Infof("channelOnMsgResponse-> \nrequestPubkey: %s, \nrequestDestPubkey: %s, \nresponsePubkey: %s, \nresponseContent: %s, time:%v\nmsgID: %s",
			requestPubkey, requestDestPubkey, responsePubkey, string(responseContent), time.Unix(timeStamp, 0), msgID)
		return nil, nil
	}
	ret.SetOnMsgRequest(channelOnRequest)
	ret.SetOnMsgResponse(channelOnResponse)
	return ret
}

func initMailService(srcPrikey *ecdsa.PrivateKey, destPrikey *ecdsa.PrivateKey) {
	dmsgService := tb.GetDmsgService()

	mailOnRequest := func(srcUserPubkey string, destUserPubkey string, msgContent []byte, timeStamp int64, msgID string, direction string) ([]byte, error) {
		decrypedContent := []byte("")
		var err error
		switch direction {
		case msg.MsgDirection.To:
			decrypedContent, err = crypto.DecryptWithPrikey(destPrikey, msgContent)
			if err != nil {
				decrypedContent = []byte(err.Error())
				logger.Errorf("decrypt error: %v", err)
			}
		case msg.MsgDirection.From:
			decrypedContent, err = crypto.DecryptWithPrikey(srcPrikey, msgContent)
			if err != nil {
				decrypedContent = []byte(err.Error())
				logger.Errorf("decrypt error: %v", err)
			}
		}
		logger.Infof("mailOnRequest-> \nsrcUserPubkey: %s, \ndestUserPubkey: %s, \nmsgContent: %s, time:%v, direction: %s",
			srcUserPubkey, destUserPubkey, string(decrypedContent), time.Unix(timeStamp, 0), direction)
		return nil, nil
	}

	dmsgService.GetMailboxService().SetOnMsgRequest(mailOnRequest)
}
