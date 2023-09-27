package main

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	ipfsLog "github.com/ipfs/go-log/v2"
	tvutilCrypto "github.com/tinyverse-web3/mtv_go_utils/crypto"
	"github.com/tinyverse-web3/mtv_go_utils/key"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	"github.com/tinyverse-web3/tvbase/dmsg/service"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

const (
	logName        = "tvnodelight"
	configFileName = "config.json"
)

var mainLog = ipfsLog.Logger(logName)

func parseCmdParams() (string, string, string, string) {
	help := flag.Bool("help", false, "Display help")
	srcSeed := flag.String("srcSeed", "", "src user pubkey")
	destSeed := flag.String("destSeed", "", "desc user pubkey")
	channelSeed := flag.String("channelSeed", "", "channel pubkey")
	rootPath := flag.String("rootPath", "", "config file path")

	flag.Parse()

	if *help {
		mainLog.Info("tinverse tvnodelight")
		mainLog.Info("Usage 1(default program run path): Run './tvnodelight -srcSeed softwarecheng@gmail.com' -destSeed softwarecheng@126.com")
		mainLog.Info("Usage 2(special data root path): Run './tvnodelight -srcSeed softwarecheng@gmail.com' -destSeed softwarecheng@126.com, -rootPath ./light1")
		os.Exit(0)
	}

	if *srcSeed == "" {
		log.Fatal("Please provide seed for generate src user public key")
	}
	if *destSeed == "" {
		log.Fatal("Please provide seed for generate dest user public key")
	}

	return *srcSeed, *destSeed, *channelSeed, *rootPath
}

func getKeyBySeed(seed string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	prikey, pubkey, err := key.GenerateEcdsaKey(seed)
	if err != nil {
		return nil, nil, err
	}
	return prikey, pubkey, nil
}

func initDmsg(
	srcPubkey *ecdsa.PublicKey,
	srcPrikey *ecdsa.PrivateKey,
	rootPath string,
	cfg *config.TvbaseConfig,
	ctx context.Context) (*tvbase.TvBase, *service.DmsgService, error) {
	tvbase, err := tvbase.NewTvbase(ctx, cfg, rootPath)
	if err != nil {
		mainLog.Fatalf("initDmsg error: %v", err)
	}

	dmsg := tvbase.GetDmsgService()
	userPubkeyData, err := key.ECDSAPublicKeyToProtoBuf(srcPubkey)
	if err != nil {
		mainLog.Errorf("initDmsg: ECDSAPublicKeyToProtoBuf error: %v", err)
		return nil, nil, err
	}

	getSig := func(protoData []byte) ([]byte, error) {
		sig, err := tvutilCrypto.SignDataByEcdsa(srcPrikey, protoData)
		if err != nil {
			mainLog.Errorf("initDmsg: sign error: %v", err)
		}
		return sig, nil
	}

	err = dmsg.Start(false, userPubkeyData, getSig, 30*time.Second)
	if err != nil {
		return nil, nil, err
	}
	return tvbase, dmsg, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init srcSeed, destSeed, rootPath from cmd params
	srcSeed, destSeed, channelSeed, rootPath := parseCmdParams()
	rootPath, err := util.GetRootPath(rootPath)
	if err != nil {
		mainLog.Fatalf("tvnode->main: GetRootPath: %v", err)
	}
	cfg, err := loadConfig(rootPath)
	if err != nil || cfg == nil {
		mainLog.Fatalf("tvnode->main: loadConfig: %v", err)
	}

	err = initLog()
	if err != nil {
		mainLog.Fatalf("tvnode->main: initLog: %v", err)
	}

	//src
	srcPrikey, srcPubkey, err := getKeyBySeed(srcSeed)
	if err != nil {
		mainLog.Errorf("getKeyBySeed error: %v", err)
		return
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPrikey))
	srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(srcPubkey))
	mainLog.Infof("src user: seed:%s, prikey:%s, pubkey:%s", srcSeed, srcPrikeyHex, srcPubkeyHex)

	//dest
	destPrikey, destPubKey, err := getKeyBySeed(destSeed)

	if err != nil {
		mainLog.Errorf("getKeyBySeed error: %v", err)
		return
	}
	destPrikeyHex := hex.EncodeToString(crypto.FromECDSA(destPrikey))
	destPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(destPubKey))

	mainLog.Infof("dest user: seed:%s, prikey:%s, pubkey:%s", destSeed, destPrikeyHex, destPubkeyHex)

	// channel
	channelPrikey, channelPubKey, err := getKeyBySeed(channelSeed)

	if err != nil {
		mainLog.Errorf("getKeyBySeed error: %v", err)
		return
	}
	channelPrikeyHex := hex.EncodeToString(crypto.FromECDSA(channelPrikey))
	channelPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(channelPubKey))

	mainLog.Infof("public user: seed:%s, prikey:%s, pubkey:%s", channelSeed, channelPrikeyHex, channelPubkeyHex)

	// init dmsg
	tvbase, dmsg, err := initDmsg(srcPubkey, srcPrikey, rootPath, cfg, ctx)
	if err != nil {
		mainLog.Errorf("initDmsg error: %v", err)
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
				mainLog.Errorf("decrypt error: %v", err)
			}
		case msg.MsgDirection.From:
			decrypedContent, err = tvutilCrypto.DecryptWithPrikey(srcPrikey, msgContent)
			if err != nil {
				decrypedContent = []byte(err.Error())
				mainLog.Errorf("decrypt error: %v", err)
			}
		}
		mainLog.Infof("msgOnRequest-> \nsrcUserPubkey: %s, \ndestUserPubkey: %s, \nmsgContent: %s, time:%v, direction: %s",
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
				mainLog.Errorf("decrypt error: %v", err)
			}
		case msg.MsgDirection.From:
			decrypedContent, err = tvutilCrypto.DecryptWithPrikey(srcPrikey, msgContent)
			if err != nil {
				decrypedContent = []byte(err.Error())
				mainLog.Errorf("decrypt error: %v", err)
			}
		}
		mainLog.Infof("mailOnRequest-> \nsrcUserPubkey: %s, \ndestUserPubkey: %s, \nmsgContent: %s, time:%v, direction: %s",
			srcUserPubkey, destUserPubkey, string(decrypedContent), time.Unix(timeStamp, 0), direction)
		return nil, nil
	}
	// set  user msg receive callback
	dmsg.GetMsgService().SetOnMsgRequest(msgOnRequest)

	msgOnResponse := func(
		requestPubkey string,
		requestDestPubkey string,
		responseDestPubkey string,
		responseContent []byte,
		timeStamp int64,
		msgID string,
	) ([]byte, error) {
		mainLog.Infof("OnMsgResponse-> \nrequestPubkey: %s, \nrequestDestPubkey: %s, \nresponseDestPubkey: %s, \nresponseContent: %s, time:%v, msgID: %s",
			requestPubkey, requestDestPubkey, responseDestPubkey, string(responseContent), time.Unix(timeStamp, 0), msgID)

		return nil, nil
	}

	dmsg.GetMsgService().SetOnMsgResponse(msgOnResponse)
	dmsg.GetMailboxService().SetOnMsgRequest(mailOnRequest)

	// publish dest user
	destPubkeyData, err := key.ECDSAPublicKeyToProtoBuf(destPubKey)
	if err != nil {
		tvbase.SetTracerStatus(err)
		mainLog.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return
	}
	destPubkeyStr := key.TranslateKeyProtoBufToString(destPubkeyData)
	err = dmsg.GetMsgService().SubscribeDestUser(destPubkeyStr)
	if err != nil {
		tvbase.SetTracerStatus(err)
		mainLog.Errorf("SubscribeDestUser error: %v", err)
		return
	}

	// publish channel
	channelPubkeyData, err := key.ECDSAPublicKeyToProtoBuf(channelPubKey)
	if err != nil {
		tvbase.SetTracerStatus(err)
		mainLog.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return
	}
	channelPubkeyStr := key.TranslateKeyProtoBufToString(channelPubkeyData)
	channelService := dmsg.GetChannelService()

	mainLog.Debugf("channelPubkeyStr: %v", channelPubkeyStr)
	err = channelService.SubscribeChannel(channelPubkeyStr)
	if err != nil {
		tvbase.SetTracerStatus(err)
		mainLog.Errorf("SubscribeChannel error: %v", err)
		return
	}
	channelOnRequest := func(
		requestPubkey string,
		requestDestPubkey string,
		requestContent []byte,
		timeStamp int64,
		msgID string,
		direction string) ([]byte, error) {
		mainLog.Infof("channelOnRequest-> \nrequestPubkey: %s, \nrequestDestPubkey: %s, \nrequestContent: %s, time:%v, direction: %s\nmsgId: %s",
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
		mainLog.Infof("channelOnMsgResponse-> \nrequestPubkey: %s, \nrequestDestPubkey: %s, \nresponsePubkey: %s, \nresponseContent: %s, time:%v\nmsgID: %s",
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
				mainLog.Errorf("read string error: %v", err)
				continue
			}
			sendContent = sendContent[:len(sendContent)-1]
			encrypedContent, err := tvutilCrypto.EncryptWithPubkey(destPubKey, []byte(sendContent))
			if err != nil {
				mainLog.Errorf("encrypt error: %v", err)
				continue
			}

			pubkeyStr := destPubkeyStr
			// pubkeyStr := channelPubkeyStr
			// encrypedContent = []byte(sendContent)
			sendMsgReq, err := dmsg.GetMsgService().SendMsg(pubkeyStr, encrypedContent)
			if err != nil {
				mainLog.Errorf("send msg: error: %v", err)
			}
			mainLog.Infof("send msg done->\nsrcPubKey:%v\ndestPubkey:%v\nid:%s, protocolID:%v, timestamp:%v,\nmsg:%v",
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
		mainLog.Infof("tvnodelight->main: received interrupt signal: %v", sig)
		cancel()
	}()

	<-ctx.Done()
	mainLog.Info("tvnodelight->main: gracefully shut down")
}

func loadConfig(rootPath string) (*config.TvbaseConfig, error) {
	ret := &config.TvbaseConfig{}

	configFilePath := rootPath + configFileName
	_, err := os.Stat(configFilePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	err = config.LoadConfig(ret, configFilePath)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func initLog() (err error) {
	var moduleLevels = map[string]string{
		"tvbase":         "debug",
		"dkvs":           "debug",
		"dmsg":           "debug",
		"customProtocol": "debug",
		"tvnode":         "debug",
		"tvipfs":         "debug",
		"core_http":      "debug",
	}
	err = util.SetLogModule(moduleLevels)
	if err != nil {
		return err
	}
	return nil
}
