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
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
	dmsg "github.com/tinyverse-web3/tvbase/dmsg"
	dmsgClient "github.com/tinyverse-web3/tvbase/dmsg/client"
	"github.com/tinyverse-web3/tvbase/tvbase"
	tvutilCrypto "github.com/tinyverse-web3/tvutil/crypto"
	tvUtilKey "github.com/tinyverse-web3/tvutil/key"
)

const logName = "tvnodelight"

var mainLog = ipfsLog.Logger(logName)

func parseCmdParams() (string, string, string, string) {
	help := flag.Bool("help", false, "Display help")
	generateCfg := flag.Bool("init", false, "init generate identityKey and config file")
	srcSeed := flag.String("srcSeed", "", "src user pubkey")
	destSeed := flag.String("destSeed", "", "desc user pubkey")
	pubSeed := flag.String("pubSeed", "", "desc user pubkey")
	rootPath := flag.String("rootPath", "", "config file path")

	flag.Parse()

	if *help {
		mainLog.Info("tinverse tvnodelight")
		mainLog.Info("Usage 1(default program run path): Run './tvnodelight -srcSeed softwarecheng@gmail.com' -destSeed softwarecheng@126.com")
		mainLog.Info("Usage 2(special data root path): Run './tvnodelight -srcSeed softwarecheng@gmail.com' -destSeed softwarecheng@126.com, -rootPath ./light1")
		os.Exit(0)
	}
	if *generateCfg {

		err := tvUtil.GenConfig2IdentityFile(*rootPath, tvConfig.LightMode)
		if err != nil {
			mainLog.Fatal(err)
		}
		os.Exit(0)
	}

	if *srcSeed == "" {
		log.Fatal("Please provide seed for generate src user public key")
	}
	if *destSeed == "" {
		log.Fatal("Please provide seed for generate dest user public key")
	}

	return *srcSeed, *destSeed, *pubSeed, *rootPath
}

func getKeyBySeed(seed string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	prikey, pubkey, err := tvUtilKey.GenerateEcdsaKey(seed)
	if err != nil {
		return nil, nil, err
	}
	return prikey, pubkey, nil
}

func initDmsg(
	srcPubkey *ecdsa.PublicKey,
	srcPrikey *ecdsa.PrivateKey,
	rootPath string,
	ctx context.Context) (*tvbase.TvBase, *dmsgClient.DmsgService, error) {
	tvInfra, err := tvbase.NewTvbase(rootPath, ctx, true)
	if err != nil {
		mainLog.Fatalf("initDmsg error: %v", err)
	}

	dmsgService := tvInfra.GetClientDmsgService()
	userPubkeyData, err := tvUtilKey.ECDSAPublicKeyToProtoBuf(srcPubkey)
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

	err = dmsgService.InitUser(userPubkeyData, getSig)
	if err != nil {
		return nil, nil, err
	}
	return tvInfra, dmsgService, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init srcSeed, destSeed, rootPath from cmd params
	srcSeed, destSeed, pubSeed, rootPath := parseCmdParams()

	nodeConfig, err := tvUtil.LoadNodeConfig(rootPath)
	if err != nil {
		mainLog.Errorf("InitConfig error: %v", err)
		return
	}
	err = tvUtil.SetLogModule(nodeConfig.Log.ModuleLevels)
	if err != nil {
		mainLog.Errorf("InitLog error: %v", err)
		return
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

	// pub
	pubPrikey, pubPubKey, err := getKeyBySeed(pubSeed)

	if err != nil {
		mainLog.Errorf("getKeyBySeed error: %v", err)
		return
	}
	pubPrikeyHex := hex.EncodeToString(crypto.FromECDSA(pubPrikey))
	pubPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(pubPubKey))

	mainLog.Infof("public user: seed:%s, prikey:%s, pubkey:%s", pubSeed, pubPrikeyHex, pubPubkeyHex)

	// init dmsg
	tvbase, dmsgService, err := initDmsg(srcPubkey, srcPrikey, rootPath, ctx)
	if err != nil {
		mainLog.Errorf("initDmsg error: %v", err)
		return
	}

	// set src user msg receive callback
	dmsgService.SetOnReceiveMsg(func(
		srcUserPubkey string,
		destUserPubkey string,
		msgContent []byte,
		timeStamp int64,
		msgID string,
		direction string) {
		decrypedContent := []byte("")

		switch direction {
		case dmsg.MsgDirection.To:
			decrypedContent, err = tvutilCrypto.DecryptWithPrikey(destPrikey, msgContent)
			if err != nil {
				decrypedContent = []byte(err.Error())
				mainLog.Errorf("decrypt error: %v", err)
			}
		case dmsg.MsgDirection.From:
			decrypedContent, err = tvutilCrypto.DecryptWithPrikey(srcPrikey, msgContent)
			if err != nil {
				decrypedContent = []byte(err.Error())
				mainLog.Errorf("decrypt error: %v", err)
			}
		}
		mainLog.Infof("OnReceiveMsg-> \nsrcUserPubkey: %s, \ndestUserPubkey: %s, \nmsgContent: %s, time:%v, direction: %s",
			srcUserPubkey, destUserPubkey, string(decrypedContent), time.Unix(timeStamp, 0), direction)
	})

	// publish dest user
	destPubkeyBytes, err := tvUtilKey.ECDSAPublicKeyToProtoBuf(destPubKey)
	if err != nil {
		tvbase.SetTracerStatus(err)
		mainLog.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return
	}
	destPubKeyStr := tvUtilKey.TranslateKeyProtoBufToString(destPubkeyBytes)
	err = dmsgService.SubscribeDestUser(destPubKeyStr)
	if err != nil {
		tvbase.SetTracerStatus(err)
		mainLog.Errorf("SubscribeDestUser error: %v", err)
		return
	}

	// publish public user
	pubPubkeyBytes, err := tvUtilKey.ECDSAPublicKeyToProtoBuf(pubPubKey)
	if err != nil {
		tvbase.SetTracerStatus(err)
		mainLog.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return
	}
	pubPubKeyStr := tvUtilKey.TranslateKeyProtoBufToString(pubPubkeyBytes)
	err = dmsgService.SubscribeChannel(pubPubKeyStr)
	if err != nil {
		tvbase.SetTracerStatus(err)
		mainLog.Errorf("SubscribePubChannel error: %v", err)
		return
	}

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

			pubkeyStr := destPubKeyStr
			// pubkeyStr := pubPubKeyStr
			sendMsgReq, err := dmsgService.SendMsg(pubkeyStr, encrypedContent)
			if err != nil {
				mainLog.Errorf("send msg: error: %v", err)
			}
			// tvcLog.Infof("sendMsgReq:%v", sendMsgReq)
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
