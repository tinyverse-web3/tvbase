package test

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"flag"
	"log"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	tvIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
	dmsgClient "github.com/tinyverse-web3/tvbase/dmsg/client"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/pullcid"
	"github.com/tinyverse-web3/tvbase/tvbase"
	tvCrypto "github.com/tinyverse-web3/tvutil/crypto"
	keyUtil "github.com/tinyverse-web3/tvutil/key"
)

func parseClientCmdParams() (string, string, string) {
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
	prikey, pubkey, err := keyUtil.GenerateEcdsaKey(seed)
	if err != nil {
		return nil, nil, err
	}
	return prikey, pubkey, nil
}

func TestPubsubMsg(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init srcSeed, destSeed, rootPath from cmd params
	srcSeed, destSeed, rootPath := parseClientCmdParams()

	nodeConfig, err := tvUtil.LoadNodeConfig(rootPath)
	if err != nil {
		tvLog.Logger.Errorf("TestPubsubMsg error: %v", err)
		return
	}
	err = tvUtil.SetLogModule(nodeConfig.Log.ModuleLevels)
	if err != nil {
		tvLog.Logger.Errorf("TestPubsubMsg error: %v", err)
		return
	}

	srcPrikey, srcPubkey, err := getKeyBySeed(srcSeed)
	if err != nil {
		tvLog.Logger.Errorf("getKeyBySeed error: %v", err)
		return
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPrikey))
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

	// init dmsg client
	_, dmsgService, err := initMsgClient(srcPubkey, srcPrikey, rootPath, ctx)
	if err != nil {
		tvLog.Logger.Errorf("init acceptable error: %v", err)
		return
	}

	// set src user msg receive callback
	dmsgService.OnReceiveMsg = func(srcUserPubkey, destUserPubkey string, msgContent []byte, timeStamp int64, msgID string, direction string) {
		tvLog.Logger.Infof("srcUserPubkey: %s, destUserPubkey: %s, msgContent: %s， time:%v, direction: %s",
			srcUserPubkey, destUserPubkey, string(msgContent), time.Unix(timeStamp, 0), direction)
	}

	// publish dest user
	destPubkeyBytes, err := keyUtil.ECDSAPublicKeyToProtoBuf(destPubKey)
	if err != nil {
		tvLog.Logger.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return
	}
	destPubKeyStr := keyUtil.TranslateKeyProtoBufToString(destPubkeyBytes)
	err = dmsgService.SubscribeDestUser(destPubKeyStr, false)
	if err != nil {
		tvLog.Logger.Errorf("SubscribeDestUser error: %v", err)
		return
	}

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

			sendMsgReq, err := dmsgService.SendMsg(destPubKeyStr, encrypedContent)

			if err != nil {
				tvLog.Logger.Errorf("send msg: error: %v", err)
			}
			tvLog.Logger.Infof("send msg: sendMsgReq: %v", sendMsgReq)

			if err != nil {
				tvLog.Logger.Infof("send msg error:", err)
			}
			tvLog.Logger.Info("SendMsg done. ")
		}
	}()

	<-ctx.Done()
}

func initMsgClient(srcPubkey *ecdsa.PublicKey, srcPrikey *ecdsa.PrivateKey, rootPath string, ctx context.Context) (*tvbase.TvBase, *dmsgClient.DmsgService, error) {
	tvbase, err := tvbase.NewTvbase(rootPath, ctx, true)
	if err != nil {
		tvLog.Logger.Errorf("InitMsgClient error: %v", err)
		return nil, nil, err
	}

	dmsgService := tvbase.GetClientDmsgService()
	srcPubkeyBytes, err := keyUtil.ECDSAPublicKeyToProtoBuf(srcPubkey)
	if err != nil {
		tvLog.Logger.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return nil, nil, err
	}

	getSigCallback := func(protoData []byte) ([]byte, error) {
		sig, err := tvCrypto.SignDataByEcdsa(srcPrikey, protoData)
		if err != nil {
			tvLog.Logger.Errorf("sign error: %v", err)
		}
		tvLog.Logger.Debugf("sign = %v", sig)
		return sig, nil
	}
	err = dmsgService.InitUser(srcPubkeyBytes, getSigCallback)
	if err != nil {
		return nil, nil, err
	}

	return tvbase, dmsgService, nil
}

func TestIpfsCmd(t *testing.T) {
	// t.Parallel()
	// t.Skip()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nodeConfig, err := tvUtil.LoadNodeConfig()
	if err != nil {
		tvLog.Logger.Errorf("TestIpfsCmd error: %v", err)
		return
	}
	err = tvUtil.SetLogModule(nodeConfig.Log.ModuleLevels)
	if err != nil {
		tvLog.Logger.Errorf("TestIpfsCmd error: %v", err)
		return
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
	srcSeed, _, rootPath := parseClientCmdParams()

	nodeConfig, err := tvUtil.LoadNodeConfig()
	if err != nil {
		tvLog.Logger.Errorf("TestPullCID error: %v", err)
		t.Errorf("InitLog error: %v", err)
		return
	}
	err = tvUtil.SetLogModule(nodeConfig.Log.ModuleLevels)
	if err != nil {
		tvLog.Logger.Errorf("TestPullCID error: %v", err)
		t.Errorf("InitLog error: %v", err)
		return
	}

	srcPrkey, srcPubkey, err := getKeyBySeed(srcSeed)
	if err != nil {
		tvLog.Logger.Errorf("getKeyBySeed error: %v", err)
		t.Errorf("getKeyBySeed error: %v", err)
		return
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPrkey))
	srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(srcPubkey))
	tvLog.Logger.Infof("src user: seed:%s, prikey:%v, pubkey:%v", srcSeed, srcPrikeyHex, srcPubkeyHex)

	// init dmsg client
	node, _, err := initMsgClient(srcPubkey, srcPrkey, rootPath, ctx)
	if err != nil {
		tvLog.Logger.Errorf("init acceptable error: %v", err)
		t.Errorf("init acceptable error: %v", err)
		return
	}

	pullCidProtocol := pullcid.GetPullCidClientProtocol()
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
	srcSeed, _, rootPath := parseClientCmdParams()

	nodeConfig, err := tvUtil.LoadNodeConfig(rootPath)
	if err != nil {
		tvLog.Logger.Errorf("TesTinverseInfrasture error: %v", err)
		t.Errorf("TesTinverseInfrasture error: %v", err)
		return
	}
	err = tvUtil.SetLogModule(nodeConfig.Log.ModuleLevels)
	if err != nil {
		tvLog.Logger.Errorf("TesTinverseInfrasture error: %v", err)
		t.Errorf("TesTinverseInfrasture error: %v", err)
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
		tvLog.Logger.Errorf("InitMsgClient error: %v", err)
	}

	<-ctx.Done()
}
