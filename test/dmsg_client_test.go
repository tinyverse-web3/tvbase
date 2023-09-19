package test

import (
	// "bufio"
	"context"
	"crypto/ecdsa"

	"flag"
	"log"
	"os"
	"testing"
	"time"

	ipfsLog "github.com/ipfs/go-log/v2"
	utilCrypto "github.com/tinyverse-web3/mtv_go_utils/crypto"
	utilKey "github.com/tinyverse-web3/mtv_go_utils/key"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	tvIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
	dmsgClient "github.com/tinyverse-web3/tvbase/dmsg/client"

	"github.com/tinyverse-web3/tvbase/tvbase"
)

const logName = "tvbase_test"

var testLog = ipfsLog.Logger(logName)

func parseCmdParams() (string, string, string) {
	help := flag.Bool("help", false, "Display help")
	generateCfg := flag.Bool("init", false, "init generate identityKey and config file")
	srcSeed := flag.String("srcSeed", "", "src user pubkey")
	destSeed := flag.String("destSeed", "", "desc user pubkey")
	rootPath := flag.String("rootPath", "", "config file path")

	flag.Parse()

	if *help {
		testLog.Info("tinverse tvnode light")
		testLog.Info("Usage 1(default program run path): Run './tvnodelight -srcSeed softwarecheng@gmail.com' -destSeed softwarecheng@126.com")
		testLog.Info("Usage 2(special data root path): Run './tvnodelight -srcSeed softwarecheng@gmail.com' -destSeed softwarecheng@126.com, -rootPath ./light1")
		os.Exit(0)
	}
	if *generateCfg {

		err := tvUtil.GenConfig2IdentityFile(*rootPath, tvConfig.LightMode)
		if err != nil {
			testLog.Fatal(err)
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
	prikey, pubkey, err := utilKey.GenerateEcdsaKey(seed)
	if err != nil {
		return nil, nil, err
	}
	return prikey, pubkey, nil
}

func TestPubsubMsg(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init srcSeed, destSeed, rootPath from cmd params
	srcSeed, _, rootPath := parseCmdParams()

	nodeConfig, err := tvUtil.LoadNodeConfig(rootPath)
	if err != nil {
		testLog.Errorf("TestPubsubMsg error: %v", err)
		return
	}
	err = tvUtil.SetLogModule(nodeConfig.Log.ModuleLevels)
	if err != nil {
		testLog.Errorf("TestPubsubMsg error: %v", err)
		return
	}

	srcPrikey, srcPubkey, err := getKeyBySeed(srcSeed)
	if err != nil {
		testLog.Errorf("getKeyBySeed error: %v", err)
		return
	}
	// srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPrikey))
	// srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(srcPubkey))
	// testLog.Infof("src user: seed:%s, prikey:%s, pubkey:%s", srcSeed, srcPrikeyHex, srcPubkeyHex)

	// destPriKey, destPubKey, err := getKeyBySeed(destSeed)
	// if err != nil {
	// 	testLog.Errorf("getKeyBySeed error: %v", err)
	// 	return
	// }
	// destPrikeyHex := hex.EncodeToString(crypto.FromECDSA(destPriKey))
	// destPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(destPubKey))

	// testLog.Infof("dest user: seed:%s, prikey:%s, pubkey:%s", destSeed, destPrikeyHex, destPubkeyHex)

	// init dmsg client
	_, dmsgService, err := initMsgClient(srcPubkey, srcPrikey, rootPath, ctx)
	if err != nil {
		testLog.Errorf("init acceptable error: %v", err)
		return
	}

	// set src user msg receive callback

	dmsgService.SetOnReceiveMsg(func(srcUserPubkey, destUserPubkey string, msgContent []byte, timeStamp int64, msgID string, direction string) {
		testLog.Infof("srcUserPubkey: %s, destUserPubkey: %s, msgContent: %s， time:%v, direction: %s",
			srcUserPubkey, destUserPubkey, string(msgContent), time.Unix(timeStamp, 0), direction)
	})

	// publish dest user
	// destPubkeyBytes, err := keyutil.ECDSAPublicKeyToProtoBuf(destPubKey)
	// if err != nil {
	// 	testLog.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
	// 	return
	// }
	// destPubKeyStr := keyutil.TranslateKeyProtoBufToString(destPubkeyBytes)
	// err = dmsgService.SubscribeDestUser(destPubKeyStr)
	// if err != nil {
	// 	testLog.Errorf("SubscribeDestUser error: %v", err)
	// 	return
	// }

	// send msg to dest user with read from stdin
	// go func() {
	// 	reader := bufio.NewReader(os.Stdin)
	// 	for {
	// 		sendContent, err := reader.ReadString('\n')
	// 		if err != nil {
	// 			testLog.Errorf("read string error: %v", err)
	// 			continue
	// 		}

	// 		encrypedContent, err := tvCrypto.EncryptWithPubkey(destPubKey, []byte(sendContent))
	// 		if err != nil {
	// 			testLog.Errorf("encrypt error: %v", err)
	// 			continue
	// 		}

	// 		sendMsgReq, err := dmsgService.SendMsg(destPubKeyStr, encrypedContent)

	// 		if err != nil {
	// 			testLog.Errorf("send msg: error: %v", err)
	// 		}
	// 		testLog.Infof("send msg: sendMsgReq: %v", sendMsgReq)

	// 		if err != nil {
	// 			testLog.Infof("send msg error:", err)
	// 		}
	// 		testLog.Info("SendMsg end")
	// 	}
	// }()

	<-ctx.Done()
}

func initMsgClient(srcPubkey *ecdsa.PublicKey, srcPrikey *ecdsa.PrivateKey, rootPath string, ctx context.Context) (*tvbase.TvBase, *dmsgClient.DmsgService, error) {
	tvbase, err := tvbase.NewTvbase(rootPath, ctx, true)
	if err != nil {
		testLog.Errorf("InitMsgClient error: %v", err)
		return nil, nil, err
	}

	dmsgService := tvbase.GetClientDmsgService()
	srcPubkeyBytes, err := utilKey.ECDSAPublicKeyToProtoBuf(srcPubkey)
	if err != nil {
		testLog.Errorf("initMsgClient: ECDSAPublicKeyToProtoBuf error: %v", err)
		return nil, nil, err
	}

	getSigCallback := func(protoData []byte) ([]byte, error) {
		sig, err := utilCrypto.SignDataByEcdsa(srcPrikey, protoData)
		if err != nil {
			testLog.Errorf("initMsgClient: sign error: %v", err)
		}
		testLog.Debugf("sign = %v", sig)
		return sig, nil
	}

	done, err := dmsgService.InitUser(srcPubkeyBytes, getSigCallback)
	if err != nil {
		return nil, nil, err
	}
	data := <-done
	if data != nil {
		testLog.Errorf("initMsgClient: InitUser error: %v", data)
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
		testLog.Errorf("TestIpfsCmd error: %v", err)
		return
	}
	err = tvUtil.SetLogModule(nodeConfig.Log.ModuleLevels)
	if err != nil {
		testLog.Errorf("TestIpfsCmd error: %v", err)
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
	testLog.Debugf("cid: %s, dataSize: %d, allElapsedTime: %v seconds, pidStatus: %v", cid, dataSize, allElapsedTime.Seconds(), pidStatus)
}

func TestPullCID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init srcSeed, destSeed, rootPath from cmd params
	// srcSeed, _, rootPath := parseClientCmdParams()

	rootPath := "."
	srcSeed := "a"
	nodeConfig, err := tvUtil.LoadNodeConfig()
	if err != nil {
		testLog.Errorf("TestPullCID error: %v", err)
		t.Errorf("InitLog error: %v", err)
		return
	}
	err = tvUtil.SetLogModule(nodeConfig.Log.ModuleLevels)
	if err != nil {
		testLog.Errorf("TestPullCID error: %v", err)
		t.Errorf("InitLog error: %v", err)
		return
	}

	srcPrkey, srcPubkey, err := getKeyBySeed(srcSeed)
	if err != nil {
		testLog.Errorf("getKeyBySeed error: %v", err)
		t.Errorf("getKeyBySeed error: %v", err)
		return
	}
	// srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPrkey))
	// srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(srcPubkey))
	// testLog.Infof("src user: seed:%s, prikey:%v, pubkey:%v", srcSeed, srcPrikeyHex, srcPubkeyHex)

	// init dmsg client
	_, _, err = initMsgClient(srcPubkey, srcPrkey, rootPath, ctx)
	if err != nil {
		testLog.Errorf("init acceptable error: %v", err)
		t.Errorf("init acceptable error: %v", err)
		return
	}

	// pullCidProtocol, err := pullcid.GetPullCidClientProtocol()
	// if err != nil {
	// 	testLog.Errorf("pullcid.GetPullCidClientProtocol error: %v", err)
	// 	t.Errorf("pullcid.GetPullCidClientProtocol error: %v", err)
	// 	return
	// }
	// err = tvbase.RegistCSCProtocol(pullCidProtocol)
	// if err != nil {
	// 	testLog.Errorf("node.RegistCSCProtocol error: %v", err)
	// 	t.Errorf("node.RegistCSCProtocol error: %v", err)
	// 	return
	// }

	// bootPeerID := "12D3KooWFvycqvSRcrPPSEygV7pU6Vd2BrpGsMMFvzeKURbGtMva"
	// localPeerID := "12D3KooWT3DqHnCgt2za47Acpf5eVxRBYgDfDZoHp7bwXTttFg7m"
	/*
		## shell for generate random cid file
		dd if=/dev/urandom of=random-file bs=1k  count=1
		## add random file to ipfs
		ipfs add ./random-file
		## check cid
		ipfs pin ls --type recursive QmPTbqArM6Pe9xbmCgcMgBFsPQFC4TFbodHTq36jrBgSVH

	*/
	// CID_RANDOM_1K := "QmdGryWJdj2pDYKNJh59cQJjaQ3Eddn8sfCVoCXS4Y639Y"
	CID_RANDOM_10M := "QmS6DqJuC3EL5TTrChAfEJvjddYVrM2X3vA3BXVNhXo4st"
	// CID_REMOTE_107_1k := "QmZ8wT2uKuQ7gv83TRwLHsqi2zDJTvB6SqKuDxkgLtYWDo"

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	info, _, err := tvIpfs.IpfsObjectStat(CID_RANDOM_10M, timeoutCtx)
	if err != nil {
		return
	}
	contentSize := (*info)[tvIpfs.ObjectStatusField_CumulativeSize]
	if contentSize >= 100*1024*1024 {
		// TODO over 100MB need to be split using CAR,, implement it
		testLog.Errorf("file too large(<100MB), bufSize:%v", contentSize)
		return
	}

	// pullCidResponseChan, err := pullCidProtocol.Request(ctx, bootPeerID, &pullcid.PullCidRequest{
	// 	CID:          CID_RANDOM_10M,
	// 	MaxCheckTime: 5 * time.Minute,
	// })
	// if err != nil {
	// 	testLog.Errorf("pullCidProtocol.Request error: %v", err)
	// 	return
	// }

	// go func() {
	// 	timeout := 30 * time.Second
	// 	select {
	// 	case pullCidResponse := <-pullCidResponseChan:
	// 		if pullCidResponse == nil {
	// 			testLog.Errorf("PullCidClientProtocol->Request: pullCidResponse is nil")
	// 			return
	// 		}
	// 		switch pullCidResponse.Status {
	// 		case tvIpfs.PinStatus_ERR:
	// 			testLog.Debugf("Save2Ipfs->PinStatus:ERR, pullCidResponse: %v", pullCidResponseChan)
	// 		case tvIpfs.PinStatus_TIMEOUT:
	// 			testLog.Debugf("Save2Ipfs->PinStatus:TIMEOUT, pullCidResponse: %v", pullCidResponseChan)
	// 		case tvIpfs.PinStatus_PINNED:
	// 			testLog.Debugf("Save2Ipfs->PinStatus:PINNED, pullCidResponse: %v", pullCidResponseChan)
	// 		default:
	// 			testLog.Debugf("Save2Ipfs->PinStatus:Other: %v, pullCidResponse: %v", pullCidResponse.Status, pullCidResponseChan)
	// 		}
	// 		testLog.Debugf("PullCidClientProtocol->Request end")
	// 		return
	// 	case <-time.After(timeout):
	// 		testLog.Debugf("PullCidClientProtocol->Request end: time.After, timeout :%v", timeout)
	// 		return
	// 	case <-ctx.Done():
	// 		testLog.Debugf("PullCidClientProtocol->Request end: ctx.Done()")
	// 		return
	// 	}
	// }()
	<-ctx.Done()
}

func TesTinverseInfrasture(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init srcSeed, destSeed, rootPath from cmd params
	srcSeed, _, rootPath := parseCmdParams()

	nodeConfig, err := tvUtil.LoadNodeConfig(rootPath)
	if err != nil {
		testLog.Errorf("TesTinverseInfrasture error: %v", err)
		t.Errorf("TesTinverseInfrasture error: %v", err)
		return
	}
	err = tvUtil.SetLogModule(nodeConfig.Log.ModuleLevels)
	if err != nil {
		testLog.Errorf("TesTinverseInfrasture error: %v", err)
		t.Errorf("TesTinverseInfrasture error: %v", err)
		return
	}

	srcPrikeyHex, srcPubkeyHex, err := getKeyBySeed(srcSeed)
	if err != nil {
		testLog.Errorf("getKeyBySeed error: %v", err)
		t.Errorf("getKeyBySeed error: %v", err)
		return
	}
	testLog.Infof("src user: seed:%s, prikey:%s, pubkey:%s", srcSeed, srcPrikeyHex, srcPubkeyHex)

	_, err = tvbase.NewTvbase(rootPath, ctx, true)
	if err != nil {
		t.Errorf("NewInfrasture error: %v", err)
		testLog.Errorf("InitMsgClient error: %v", err)
	}

	<-ctx.Done()
}
