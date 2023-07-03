package main

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	ipfsLog "github.com/ipfs/go-log/v2"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
	dmsgClient "github.com/tinyverse-web3/tvbase/dmsg/client"
	"github.com/tinyverse-web3/tvbase/tvbase"
	tvcrypto "github.com/tinyverse-web3/tvutil/crypto"
	keyutil "github.com/tinyverse-web3/tvutil/key"
)

const logName = "tvnodelight"

var tvcLog = ipfsLog.Logger(logName)

func init() {
	ipfsLog.SetLogLevel(logName, "debug")
}

func parseCmdParams() (string, string, string) {
	help := flag.Bool("help", false, "Display help")
	generateCfg := flag.Bool("init", false, "init generate identityKey and config file")
	srcSeed := flag.String("srcSeed", "", "src user pubkey")
	destSeed := flag.String("destSeed", "", "desc user pubkey")
	rootPath := flag.String("rootPath", "", "config file path")

	flag.Parse()

	if *help {
		tvcLog.Info("tinverse tvnodelight")
		tvcLog.Info("Usage 1(default program run path): Run './tvnodelight -srcSeed softwarecheng@gmail.com' -destSeed softwarecheng@126.com")
		tvcLog.Info("Usage 2(special data root path): Run './tvnodelight -srcSeed softwarecheng@gmail.com' -destSeed softwarecheng@126.com, -rootPath ./light1")
		os.Exit(0)
	}
	if *generateCfg {

		err := tvUtil.GenConfig2IdentityFile(*rootPath, tvConfig.LightMode)
		if err != nil {
			tvcLog.Fatal(err)
		}
		os.Exit(0)
	}

	if *srcSeed == "" {
		log.Fatal("Please provide seed for generate src user public key")
	}
	if *destSeed == "" {
		log.Fatal("Please provide seed for generate dest user public key")
	}

	return *srcSeed, *destSeed, *rootPath
}

func getKeyBySeed(seed string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	prikey, pubkey, err := keyutil.GenerateEcdsaKey(seed)
	if err != nil {
		return nil, nil, err
	}
	return prikey, pubkey, nil
}

func initMsgClient(srcPubKey *ecdsa.PublicKey, rootPath string, ctx context.Context) (*tvbase.Tvbase, *dmsgClient.DmsgService, error) {
	tvInfra, err := tvbase.NewTvbase(rootPath, ctx, true)
	if err != nil {
		tvcLog.Fatalf("InitMsgClient error: %v", err)
	}

	dmsgService := tvInfra.GetClientDmsgService()
	srcPubkeyStr, err := keyutil.ECDSAPublicKeyToProtoBuf(srcPubKey)
	if err != nil {
		tvcLog.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return nil, nil, err
	}
	err = dmsgService.InitUser(srcPubkeyStr)
	if err != nil {
		return nil, nil, err
	}

	return tvInfra, dmsgService, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init srcSeed, destSeed, rootPath from cmd params
	srcSeed, destSeed, rootPath := parseCmdParams()

	err := tvUtil.InitConfig(rootPath)
	if err != nil {
		tvcLog.Errorf("InitConfig error: %v", err)
		return
	}
	err = tvUtil.InitLog()
	if err != nil {
		tvcLog.Errorf("InitLog error: %v", err)
		return
	}

	srcPriKey, srcPubkey, err := getKeyBySeed(srcSeed)
	if err != nil {
		tvcLog.Errorf("getKeyBySeed error: %v", err)
		return
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPriKey))
	srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(srcPubkey))
	tvcLog.Infof("src user: seed:%s, prikey:%s, pubkey:%s", srcSeed, srcPrikeyHex, srcPubkeyHex)

	destPriKey, destPubKey, err := getKeyBySeed(destSeed)
	if err != nil {
		tvcLog.Errorf("getKeyBySeed error: %v", err)
		return
	}
	destPrikeyHex := hex.EncodeToString(crypto.FromECDSA(destPriKey))
	destPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(destPubKey))

	tvcLog.Infof("dest user: seed:%s, prikey:%s, pubkey:%s", destSeed, destPrikeyHex, destPubkeyHex)

	// init dmsg client
	tvbase, dmsgService, err := initMsgClient(srcPubkey, rootPath, ctx)
	if err != nil {
		tvcLog.Errorf("init acceptable error: %v", err)
		return
	}

	// set src user msg receive callback
	dmsgService.OnReceiveMsg = func(srcUserPubkey, destUserPubkey string, msgContent []byte, timeStamp int64, msgID string, direction string) {
		tvcLog.Infof("srcUserPubkey: %s, destUserPubkey: %s, msgContent: %s， time:%v, direction: %s",
			srcUserPubkey, destUserPubkey, string(msgContent), time.Unix(timeStamp, 0), direction)
	}

	// publish dest user
	destPubkeyBytes, err := keyutil.ECDSAPublicKeyToProtoBuf(destPubKey)
	if err != nil {
		tvbase.SetTracerStatus(err)
		tvcLog.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return
	}
	destPubKeyStr := keyutil.TranslateKeyProtoBufToString(destPubkeyBytes)
	err = dmsgService.SubscribeDestUser(destPubKeyStr)
	if err != nil {
		tvbase.SetTracerStatus(err)
		tvcLog.Errorf("SubscribeDestUser error: %v", err)
		return
	}

	// send msg to dest user with read from stdin
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			sendContent, err := reader.ReadString('\n')
			if err != nil {
				tvcLog.Errorf("read string error: %v", err)
				continue
			}

			encrypedContent, err := tvcrypto.EncryptWithPubkey(destPubKey, []byte(sendContent))
			if err != nil {
				tvcLog.Errorf("encrypt error: %v", err)
				continue
			}
			// Prepare the message info for sending, requestMsg is struct data for sending, data is signning data
			requestMsg, data, err := dmsgService.PreSendMsg(destPubKeyStr, encrypedContent)
			if err != nil {
				tvcLog.Errorf("preSendMsg error: %v", err)
				continue
			}
			tvcLog.Debugf("sign content = %v", data)
			// sign the message data by account center

			signed, err := tvcrypto.SignDataByEcdsa(srcPriKey, data)
			if err != nil {
				tvcLog.Errorf("sign error: %v", err)
				continue
			}

			tvcLog.Debugf("sign = %v", signed)
			// Send the message with sign
			err = dmsgService.SendMsg(requestMsg, signed)
			tvcLog.Info("SendMsg done. ")

			if err != nil {
				fmt.Println("send msg error:", err)
			}
		}
	}()

	<-ctx.Done()
	tvcLog.Info("tvnodelight->main: Gracefully shut down")
	tvbase.Stop()
}
