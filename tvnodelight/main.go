package main

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	tvutilCrypto "github.com/tinyverse-web3/mtv_go_utils/crypto"
	"github.com/tinyverse-web3/mtv_go_utils/key"
	"github.com/tinyverse-web3/tvbase/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

func startDmsgService(srcPubkey *ecdsa.PublicKey, srcPrikey *ecdsa.PrivateKey, tb *tvbase.TvBase) error {
	userPubkeyData, err := key.ECDSAPublicKeyToProtoBuf(srcPubkey)
	if err != nil {
		logger.Errorf("initDmsg: ECDSAPublicKeyToProtoBuf error: %v", err)
		return err
	}

	getSig := func(protoData []byte) ([]byte, error) {
		sig, err := tvutilCrypto.SignDataByEcdsa(srcPrikey, protoData)
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
	srcSeed, destSeed, channelSeed, rootPath := parseCmdParams()
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

	//src
	srcPrikey, srcPubkey, err := getKeyBySeed(srcSeed)
	if err != nil {
		logger.Errorf("getKeyBySeed error: %v", err)
		return
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPrikey))
	srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(srcPubkey))
	logger.Infof("src user: seed:%s, prikey:%s, pubkey:%s", srcSeed, srcPrikeyHex, srcPubkeyHex)

	//dest
	destPrikey, destPubKey, err := getKeyBySeed(destSeed)

	if err != nil {
		logger.Errorf("getKeyBySeed error: %v", err)
		return
	}
	destPrikeyHex := hex.EncodeToString(crypto.FromECDSA(destPrikey))
	destPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(destPubKey))

	logger.Infof("dest user: seed:%s, prikey:%s, pubkey:%s", destSeed, destPrikeyHex, destPubkeyHex)

	// channel
	channelPrikey, channelPubKey, err := getKeyBySeed(channelSeed)

	if err != nil {
		logger.Errorf("getKeyBySeed error: %v", err)
		return
	}
	channelPrikeyHex := hex.EncodeToString(crypto.FromECDSA(channelPrikey))
	channelPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(channelPubKey))

	logger.Infof("public user: seed:%s, prikey:%s, pubkey:%s", channelSeed, channelPrikeyHex, channelPubkeyHex)

	// init dmsgService
	tb, err := tvbase.NewTvbase(ctx, cfg, rootPath)
	if err != nil {
		logger.Fatalf("initDmsg error: %v", err)
	}
	tb.Start()
	err = startDmsgService(srcPubkey, srcPrikey, tb)
	if err != nil {
		logger.Errorf("initDmsg error: %v", err)
		return
	}

	msgOnRequest := func(
		srcUserPubkey string,
		destUserPubkey string,
		msgContent []byte,
		timeStamp int64,
		msgID string,
		direction string) ([]byte, error) {
		decrypedContent := []byte("")

		switch direction {
		case msg.MsgDirection.To:
			decrypedContent, err = tvutilCrypto.DecryptWithPrikey(destPrikey, msgContent)
			if err != nil {
				decrypedContent = []byte(err.Error())
				logger.Errorf("decrypt error: %v", err)
			}
		case msg.MsgDirection.From:
			decrypedContent, err = tvutilCrypto.DecryptWithPrikey(srcPrikey, msgContent)
			if err != nil {
				decrypedContent = []byte(err.Error())
				logger.Errorf("decrypt error: %v", err)
			}
		}
		logger.Infof("msgOnRequest-> \nsrcUserPubkey: %s, \ndestUserPubkey: %s, \nmsgContent: %s, time:%v, direction: %s",
			srcUserPubkey, destUserPubkey, string(decrypedContent), time.Unix(timeStamp, 0), direction)
		return nil, nil
	}

	mailOnRequest := func(
		srcUserPubkey string,
		destUserPubkey string,
		msgContent []byte,
		timeStamp int64,
		msgID string,
		direction string) ([]byte, error) {
		decrypedContent := []byte("")

		switch direction {
		case msg.MsgDirection.To:
			decrypedContent, err = tvutilCrypto.DecryptWithPrikey(destPrikey, msgContent)
			if err != nil {
				decrypedContent = []byte(err.Error())
				logger.Errorf("decrypt error: %v", err)
			}
		case msg.MsgDirection.From:
			decrypedContent, err = tvutilCrypto.DecryptWithPrikey(srcPrikey, msgContent)
			if err != nil {
				decrypedContent = []byte(err.Error())
				logger.Errorf("decrypt error: %v", err)
			}
		}
		logger.Infof("mailOnRequest-> \nsrcUserPubkey: %s, \ndestUserPubkey: %s, \nmsgContent: %s, time:%v, direction: %s",
			srcUserPubkey, destUserPubkey, string(decrypedContent), time.Unix(timeStamp, 0), direction)
		return nil, nil
	}

	dmsgService := tb.GetDmsgService()
	// set  user msg receive callback
	dmsgService.GetMsgService().SetOnMsgRequest(msgOnRequest)

	msgOnResponse := func(
		requestPubkey string,
		requestDestPubkey string,
		responseDestPubkey string,
		responseContent []byte,
		timeStamp int64,
		msgID string,
	) ([]byte, error) {
		logger.Infof("OnMsgResponse-> \nrequestPubkey: %s, \nrequestDestPubkey: %s, \nresponseDestPubkey: %s, \nresponseContent: %s, time:%v, msgID: %s",
			requestPubkey, requestDestPubkey, responseDestPubkey, string(responseContent), time.Unix(timeStamp, 0), msgID)

		return nil, nil
	}

	dmsgService.GetMsgService().SetOnMsgResponse(msgOnResponse)
	dmsgService.GetMailboxService().SetOnMsgRequest(mailOnRequest)

	// publish dest user
	destPubkeyData, err := key.ECDSAPublicKeyToProtoBuf(destPubKey)
	if err != nil {
		tb.SetTracerStatus(err)
		logger.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return
	}
	destPubkeyStr := key.TranslateKeyProtoBufToString(destPubkeyData)
	err = dmsgService.GetMsgService().SubscribeDestUser(destPubkeyStr)
	if err != nil {
		tb.SetTracerStatus(err)
		logger.Errorf("SubscribeDestUser error: %v", err)
		return
	}

	// publish channel
	channelPubkeyData, err := key.ECDSAPublicKeyToProtoBuf(channelPubKey)
	if err != nil {
		tb.SetTracerStatus(err)
		logger.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return
	}
	channelPubkeyStr := key.TranslateKeyProtoBufToString(channelPubkeyData)
	channelService := dmsgService.GetChannelService()

	logger.Debugf("channelPubkeyStr: %v", channelPubkeyStr)
	err = channelService.SubscribeChannel(channelPubkeyStr)
	if err != nil {
		tb.SetTracerStatus(err)
		logger.Errorf("SubscribeChannel error: %v", err)
		return
	}
	channelOnRequest := func(
		requestPubkey string,
		requestDestPubkey string,
		requestContent []byte,
		timeStamp int64,
		msgID string,
		direction string) ([]byte, error) {
		logger.Infof("channelOnRequest-> \nrequestPubkey: %s, \nrequestDestPubkey: %s, \nrequestContent: %s, time:%v, direction: %s\nmsgId: %s",
			requestPubkey, requestDestPubkey, string(requestContent), time.Unix(timeStamp, 0), direction, msgID)
		return nil, nil
	}
	channelOnResponse := func(
		requestPubkey string,
		requestDestPubkey string,
		responsePubkey string,
		responseContent []byte,
		timeStamp int64,
		msgID string) ([]byte, error) {
		logger.Infof("channelOnMsgResponse-> \nrequestPubkey: %s, \nrequestDestPubkey: %s, \nresponsePubkey: %s, \nresponseContent: %s, time:%v\nmsgID: %s",
			requestPubkey, requestDestPubkey, responsePubkey, string(responseContent), time.Unix(timeStamp, 0), msgID)
		return nil, nil
	}
	channelService.SetOnMsgRequest(channelOnRequest)
	channelService.SetOnMsgResponse(channelOnResponse)

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
			encrypedContent, err := tvutilCrypto.EncryptWithPubkey(destPubKey, []byte(sendContent))
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
