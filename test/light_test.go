package test

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	tvIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
	dmsglight "github.com/tinyverse-web3/tvbase/dmsg/light"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/pullcid"
	"github.com/tinyverse-web3/tvbase/tvbase"
	tvCrypto "github.com/tinyverse-web3/tvutil/crypto"
	keyUtils "github.com/tinyverse-web3/tvutil/key"
)

func parseLightCmdParams() (string, string, string) {
	help := flag.Bool("help", false, "Display help")
	generateCfg := flag.Bool("init", false, "init generate identityKey and config file")
	srcSeed := flag.String("srcSeed", "", "src user pubkey")
	destSeed := flag.String("destSeed", "", "desc user pubkey")
	rootPath := flag.String("rootPath", "", "config file path")

	flag.Parse()

	if *help {
		tvLog.Logger.Info("tinverse tvnode light")
		tvLog.Logger.Info("Usage 1(default program run path): Run './tvnodelight -srcSeed softwarecheng@gmail.com' -destSeed softwarecheng@126.com")
		tvLog.Logger.Info("Usage 2(special data root path): Run './tvnodelight -srcSeed softwarecheng@gmail.com' -destSeed softwarecheng@126.com, -rootPath ./light1")
		os.Exit(0)
	}
	if *generateCfg {

		err := tvUtil.GenConfig2IdentityFile(*rootPath, tvConfig.LightMode)
		if err != nil {
			tvLog.Logger.Fatal(err)
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
	prikey, pubkey, err := keyUtils.GenerateEcdsaKey(seed)
	if err != nil {
		return nil, nil, err
	}
	return prikey, pubkey, nil
}

func TestPubsubMsg(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init srcSeed, destSeed, rootPath from cmd params
	srcSeed, destSeed, rootPath := parseLightCmdParams()

	err := tvUtil.InitLog(rootPath)
	if err != nil {
		tvLog.Logger.Errorf("InitLog error: %v", err)
		return
	}

	srcPriKey, srcPubkey, err := getKeyBySeed(srcSeed)
	if err != nil {
		tvLog.Logger.Errorf("getKeyBySeed error: %v", err)
		return
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPriKey))
	srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(srcPubkey))
	tvLog.Logger.Infof("src user: seed:%s, prikey:%s, pubkey:%s", srcSeed, srcPrikeyHex, srcPubkeyHex)

	destPriKey, destPubKey, err := getKeyBySeed(destSeed)
	if err != nil {
		tvLog.Logger.Errorf("getKeyBySeed error: %v", err)
		return
	}
	destPrikeyHex := hex.EncodeToString(crypto.FromECDSA(destPriKey))
	destPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(destPubKey))

	tvLog.Logger.Infof("dest user: seed:%s, prikey:%s, pubkey:%s", destSeed, destPrikeyHex, destPubkeyHex)

	// init dmsg light
	_, dmsgLight, err := initMsgLight(srcPubkey, rootPath, ctx)
	if err != nil {
		tvLog.Logger.Errorf("init acceptable error: %v", err)
		return
	}

	// set src user msg receive callback
	dmsgLight.OnReceiveMsg = func(srcUserPubkey, destUserPubkey string, msgContent []byte, timeStamp int64, msgID string, direction string) {
		tvLog.Logger.Infof("srcUserPubkey: %s, destUserPubkey: %s, msgContent: %s， time:%v, direction: %s",
			srcUserPubkey, destUserPubkey, string(msgContent), time.Unix(timeStamp, 0), direction)
	}

	// get src user message list
	// msgList, err := dmsgLight.GetUserMsgList(destPubkeyHex)
	// if err != nil {
	// 	tvLog.Logger.Errorf("GetUserMsgList error: %v", err)
	// 	return
	// }
	// tvLog.Logger.Info("show src user message list:")
	// for _, v := range msgList {
	// 	tvLog.Logger.Infof("msg: %v", v)
	// }

	// publish dest user
	destPubkeyBytes, err := keyUtils.ECDSAPublicKeyToProtoBuf(destPubKey)
	if err != nil {
		tvLog.Logger.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return
	}
	destPubKeyStr := keyUtils.TranslateKeyProtoBufToString(destPubkeyBytes)
	err = dmsgLight.SubscribeDestUser(destPubKeyStr)
	if err != nil {
		tvLog.Logger.Errorf("SubscribeDestUser error: %v", err)
		return
	}

	// ticker send msg and publish incorrect protocol by pubsub
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// dmsgLight.SendMsg(destPubkeyHex, []byte("hello"))
				// fmt.Println("Executing every 2 seconds...")

				//send err protocol data
				// pubsub := dmsgLight.DestUserPubsubs[destPubkeyHex]
				// topic := pubsub.UserSub.Topic()
				// tvLog.Logger.Infof("pubsub topic: %s", topic)
				// pubsub.UserTopic.Publish(dmsgLight.DmsgService.Ctx, []byte("hello"))
			case <-dmsgLight.DmsgService.NodeService.GetCtx().Done():
				return
			}
		}
	}()

	// send msg to dest user with read from stdin
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			sendContent, err := reader.ReadString('\n')
			if err != nil {
				tvLog.Logger.Errorf("read string error: %v", err)
				continue
			}

			encrypedContent, err := tvCrypto.EncryptWithPubkey(destPubKey, []byte(sendContent))
			if err != nil {
				tvLog.Logger.Errorf("encrypt error: %v", err)
				continue
			}
			// Prepare the message info for sending, requestMsg is struct data for sending, data is signning data
			requestMsg, data, err := dmsgLight.PreSendMsg(destPubKeyStr, encrypedContent)
			if err != nil {
				tvLog.Logger.Errorf("preSendMsg error: %v", err)
				continue
			}

			tvLog.Logger.Debugf("sign content = %v", data)
			// sign the message data by account center

			signed, err := tvCrypto.SignDataByEcdsa(srcPriKey, data)
			if err != nil {
				tvLog.Logger.Errorf("sign error: %v", err)
				continue
			}

			tvLog.Logger.Debugf("sign = %v", signed)
			// Send the message with sign
			err = dmsgLight.SendMsg(requestMsg, signed)
			tvLog.Logger.Info("SendMsg done. ")

			if err != nil {
				fmt.Println("send msg error:", err)
			}
		}
	}()

	<-ctx.Done()
}

func initMsgLight(srcPubKey *ecdsa.PublicKey, rootPath string, ctx context.Context) (*tvbase.Tvbase, *dmsglight.DmsgService, error) {
	tvbase, err := tvbase.NewTvbase(rootPath, ctx, true)
	if err != nil {
		tvLog.Logger.Fatalf("InitMsgLight error: %v", err)
	}

	dmsgLight := tvbase.GetLightDmsgService()
	srcPubkeyStr, err := keyUtils.ECDSAPublicKeyToProtoBuf(srcPubKey)
	if err != nil {
		tvLog.Logger.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return nil, nil, err
	}
	err = dmsgLight.InitUser(srcPubkeyStr)
	if err != nil {
		return nil, nil, err
	}

	return tvbase, dmsgLight, nil
}

func TestIpfsCmd(t *testing.T) {
	// t.Parallel()
	// t.Skip()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := tvUtil.InitLog("")
	if err != nil {
		tvLog.Logger.Fatalf("InitLog error: %v", err)
	}

	err = tvIpfs.CheckIpfsCmd()
	if err != nil {
		t.Error(err)
		return
	}

	cid := "QmTX7d5vWYrmKzj35MwcEJYsrA6P7Uf6ieWWNJf7kdjdX4"
	timeout := 5 * time.Minute
	// timeout = 1 * time.Second
	dataSize, allElapsedTime, pidStatus, err := tvIpfs.IpfsGetObject(cid, ctx, timeout)
	if err != nil {
		t.Error(err)
		return
	}
	tvLog.Logger.Debugf("cid: %s, dataSize: %d, allElapsedTime: %v seconds, pidStatus: %v", cid, dataSize, allElapsedTime.Seconds(), pidStatus)
}

func TestPullCID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init srcSeed, destSeed, rootPath from cmd params
	srcSeed, _, rootPath := parseLightCmdParams()

	err := tvUtil.InitLog(rootPath)
	if err != nil {
		tvLog.Logger.Errorf("InitLog error: %v", err)
		t.Errorf("InitLog error: %v", err)
		return
	}

	srcPriKey, srcPubKey, err := getKeyBySeed(srcSeed)
	if err != nil {
		tvLog.Logger.Errorf("getKeyBySeed error: %v", err)
		t.Errorf("getKeyBySeed error: %v", err)
		return
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPriKey))
	srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(srcPubKey))
	tvLog.Logger.Infof("src user: seed:%s, prikey:%v, pubkey:%v", srcSeed, srcPrikeyHex, srcPubkeyHex)

	// init dmsg light
	node, _, err := initMsgLight(srcPubKey, rootPath, ctx)
	if err != nil {
		tvLog.Logger.Errorf("init acceptable error: %v", err)
		t.Errorf("init acceptable error: %v", err)
		return
	}

	pullCidProtocol := pullcid.GetPullCidLightProtocol()
	err = node.RegistCSCProtocol(pullCidProtocol)
	if err != nil {
		tvLog.Logger.Errorf("node.RegistCSCProtocol error: %v", err)
		t.Errorf("node.RegistCSCProtocol error: %v", err)
		return
	}
	pullCidResponse, err := pullCidProtocol.Request(&pullcid.PullCidRequest{
		CID:          "QmTX7d5vWYrmKzj35MwcEJYsrA6P7Uf6ieWWNJf7kdjdX4",
		CheckTimeout: 5 * time.Minute,
	})
	if err != nil {
		tvLog.Logger.Errorf("pullCidProtocol.Request error: %v", err)
		t.Errorf("pullCidProtocol.Request error: %v", err)
	}
	tvLog.Logger.Infof("pullCidResponse: %v", pullCidResponse)

	<-ctx.Done()
}

func TesTinverseInfrasture(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init srcSeed, destSeed, rootPath from cmd params
	srcSeed, _, rootPath := parseLightCmdParams()

	err := tvUtil.InitLog(rootPath)
	if err != nil {
		tvLog.Logger.Errorf("InitLog error: %v", err)
		t.Errorf("InitLog error: %v", err)
		return
	}

	srcPrikeyHex, srcPubkeyHex, err := getKeyBySeed(srcSeed)
	if err != nil {
		tvLog.Logger.Errorf("getKeyBySeed error: %v", err)
		t.Errorf("getKeyBySeed error: %v", err)
		return
	}
	tvLog.Logger.Infof("src user: seed:%s, prikey:%s, pubkey:%s", srcSeed, srcPrikeyHex, srcPubkeyHex)

	_, err = tvbase.NewTvbase(rootPath, ctx, true)
	if err != nil {
		t.Errorf("NewInfrasture error: %v", err)
		tvLog.Logger.Errorf("InitMsgLight error: %v", err)
		//tvLog.Logger.Fatalf("InitMsgLight error: %v", err)
	}

	<-ctx.Done()
}
