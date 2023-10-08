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
	ipfsLog "github.com/ipfs/go-log/v2"
	cryptoUtil "github.com/tinyverse-web3/mtv_go_utils/crypto"
	"github.com/tinyverse-web3/mtv_go_utils/ipfs"
	"github.com/tinyverse-web3/mtv_go_utils/key"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

const (
	logName        = "tvbase_test"
	configFileName = "config.json"
)

var testLog = ipfsLog.Logger(logName)

func parseCmdParams() (string, string, string) {
	help := flag.Bool("help", false, "Display help")
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

	if *srcSeed == "" {
		log.Fatal("Please provide seed for generate src user public key")
	}
	if *destSeed == "" {
		log.Fatal("Please provide seed for generate dest user public key")
	}

	return *srcSeed, *destSeed, *rootPath
}

func getKeyBySeed(seed string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	prikey, pubkey, err := key.GenerateEcdsaKey(seed)
	if err != nil {
		return nil, nil, err
	}
	return prikey, pubkey, nil
}

func TestPubsubMsg(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init srcSeed, destSeed, rootPath from cmd params
	srcSeed, destSeed, rootPath := parseCmdParams()

	cfg, err := loadConfig(rootPath)
	if err != nil {
		testLog.Errorf("TestPubsubMsg error: %v", err)
		return
	}

	err = initLog()
	if err != nil {
		testLog.Fatalf("tvnode->main: initLog: %v", err)
	}

	srcPrikey, srcPubkey, err := getKeyBySeed(srcSeed)
	if err != nil {
		testLog.Errorf("getKeyBySeed error: %v", err)
		return
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPrikey))
	srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(srcPubkey))
	testLog.Infof("src user: seed:%s, prikey:%s, pubkey:%s", srcSeed, srcPrikeyHex, srcPubkeyHex)

	destPriKey, destPubKey, err := getKeyBySeed(destSeed)
	if err != nil {
		testLog.Errorf("getKeyBySeed error: %v", err)
		return
	}
	destPrikeyHex := hex.EncodeToString(crypto.FromECDSA(destPriKey))
	destPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(destPubKey))

	testLog.Infof("dest user: seed:%s, prikey:%s, pubkey:%s", destSeed, destPrikeyHex, destPubkeyHex)

	// init dmsg client
	_, dmsg, err := initService(srcPubkey, srcPrikey, cfg, rootPath, ctx)
	if err != nil {
		testLog.Errorf("init acceptable error: %v", err)
		return
	}

	// set src user msg receive callback
	onReceiveMsg := func(message *msg.ReceiveMsg) ([]byte, error) {
		testLog.Infof("srcUserPubkey: %s, destUserPubkey: %s, msgContent: %s， time:%v, direction: %s",
			message.ReqPubkey, message.DestPubkey, string(message.Content), time.Unix(message.TimeStamp, 0), message.Direction)
		return nil, nil
	}
	dmsg.GetMsgService().SetOnReceiveMsg(onReceiveMsg)
	dmsg.GetMailboxService().SetOnReceiveMsg(onReceiveMsg)
	// publish dest user
	destPubkeyBytes, err := key.ECDSAPublicKeyToProtoBuf(destPubKey)
	if err != nil {
		testLog.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return
	}
	destPubKeyStr := key.TranslateKeyProtoBufToString(destPubkeyBytes)
	err = dmsg.GetMsgService().SubscribeDestUser(destPubKeyStr)
	if err != nil {
		testLog.Errorf("SubscribeDestUser error: %v", err)
		return
	}

	// send msg to dest user with read from stdin
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			sendContent, err := reader.ReadString('\n')
			if err != nil {
				testLog.Errorf("read string error: %v", err)
				continue
			}

			encrypedContent, err := cryptoUtil.EncryptWithPubkey(destPubKey, []byte(sendContent))
			if err != nil {
				testLog.Errorf("encrypt error: %v", err)
				continue
			}

			sendMsgReq, err := dmsg.GetMsgService().SendMsg(destPubKeyStr, encrypedContent)

			if err != nil {
				testLog.Errorf("send msg: error: %v", err)
			}
			testLog.Infof("send msg: sendMsgReq: %v", sendMsgReq)

			if err != nil {
				testLog.Infof("send msg error:", err)
			}
			testLog.Info("SendMsg end")
		}
	}()

	<-ctx.Done()
}

func initService(
	srcPubkey *ecdsa.PublicKey,
	srcPrikey *ecdsa.PrivateKey,
	cfg *config.TvbaseConfig,
	rootPath string,
	ctx context.Context) (*tvbase.TvBase, *DmsgService, error) {
	tb, err := tvbase.NewTvbase(ctx, cfg, rootPath)
	if err != nil {
		testLog.Errorf("initService error: %v", err)
		return nil, nil, err
	}

	srcPubkeyBytes, err := key.ECDSAPublicKeyToProtoBuf(srcPubkey)
	if err != nil {
		testLog.Errorf("initMsgClient: ECDSAPublicKeyToProtoBuf error: %v", err)
		return nil, nil, err
	}

	getSigCallback := func(protoData []byte) ([]byte, error) {
		sig, err := cryptoUtil.SignDataByEcdsa(srcPrikey, protoData)
		if err != nil {
			testLog.Errorf("initMsgClient: sign error: %v", err)
		}
		testLog.Debugf("sign = %v", sig)
		return sig, nil
	}

	dmsgService, err := CreateDmsgService(tb)
	if err != nil {
		return nil, nil, err
	}

	err = dmsgService.Start(false, srcPubkeyBytes, getSigCallback, 30*time.Second)
	if err != nil {
		return nil, nil, err
	}

	return tb, dmsgService, nil
}

func TestPullCID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rootPath := "."
	srcSeed := "a"
	cfg, err := loadConfig("./")
	if err != nil {
		testLog.Errorf("TestPullCID error: %v", err)
		return
	}
	err = initLog()
	if err != nil {
		testLog.Errorf("TestPullCID error: %v", err)
		return
	}

	srcPrkey, srcPubkey, err := getKeyBySeed(srcSeed)
	if err != nil {
		testLog.Errorf("getKeyBySeed error: %v", err)
		return
	}
	srcPrikeyHex := hex.EncodeToString(crypto.FromECDSA(srcPrkey))
	srcPubkeyHex := hex.EncodeToString(crypto.FromECDSAPub(srcPubkey))
	testLog.Infof("src user: seed:%s, prikey:%v, pubkey:%v", srcSeed, srcPrikeyHex, srcPubkeyHex)

	// init dmsg client
	tb, _, err := initService(srcPubkey, srcPrkey, cfg, rootPath, ctx)
	if err != nil {
		testLog.Errorf("init acceptable error: %v", err)
		return
	}

	// pullCidProtocol, err := pullcid.GetPullCidClientProtocol()
	// if err != nil {
	// 	testLog.Errorf("pullcid.GetPullCidClientProtocol error: %v", err)
	// 	return
	// }

	// err = tvbase.GetDmsgService().GetCustomProtocolService().RegistClient(pullCidProtocol)
	// if err != nil {
	// 	testLog.Errorf("RegistClient error: %v", err)
	// 	return
	// }

	dmsgService, err := CreateDmsgService(tb)
	if err != nil {
		testLog.Errorf("CreateDmsgService error: %v", err)
		return
	}

	queryPeerRequest, queryPeerResponseChan, err := dmsgService.GetCustomProtocolService().QueryPeer("pullcid")
	if err != nil {
		testLog.Errorf("QueryPeer error: %v", err)
		return
	}
	testLog.Debugf("queryPeerRequest: %v", queryPeerRequest)
	queryPeerResponseData := <-queryPeerResponseChan
	queryPeerResponse, ok := queryPeerResponseData.(*pb.QueryPeerRes)
	if !ok {
		testLog.Errorf("QueryPeerRes error: %v", err)
		return
	}
	testLog.Debugf("queryPeerResponse: %v", queryPeerResponse)
	// peerId := queryPeerResponse.BasicData.PeerID
	// bootPeerID := "12D3KooWFvycqvSRcrPPSEygV7pU6Vd2BrpGsMMFvzeKURbGtMva"
	// localPeerID := "12D3KooWDHUopoYJemJxzMSrTFPpshbKFaEJv3xX1SogvZpcMEic"
	/*
		## shell for generate random cid file
		dd if=/dev/urandom of=random-file bs=1k  count=1
		## add random file to ipfs
		ipfs add ./random-file
		## check cid
		ipfs pin ls --type recursive QmPTbqArM6Pe9xbmCgcMgBFsPQFC4TFbodHTq36jrBgSVH
	*/

	CID_RANDOM_1K := "QmfTpubWPxpiWy8LDi3XKKgirpJQqkobhxvHR1UVabkezz"
	CID_RANDOM_1M := "QmadujPGectw6pm7Sx9McePenrUohpLfzE4FxhnzHCM6vS"
	CID_RANDOM_10M := "QmVa9N59PRDeqTSV6L3wRTMDfU5vycviBky41q6aGyJZmX"
	cid := CID_RANDOM_1K
	cid = CID_RANDOM_1M
	cid = CID_RANDOM_10M
	cid = CID_RANDOM_10M

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	info, _, err := ipfs.IpfsObjectStat(cid, timeoutCtx)
	if err != nil {
		return
	}
	contentSize := (*info)[ipfs.ObjectStatusField_CumulativeSize]
	if contentSize >= 100*1024*1024 {
		// TODO over 100MB need to be split using CAR,, implement it
		testLog.Errorf("file too large(<100MB), bufSize:%v", contentSize)
		return
	}

	// pullCidResponseChan, err := pullCidProtocol.Request(ctx, peerId, &pullcid.PullCidRequest{
	// 	CID:          cid,
	// 	MaxCheckTime: 30 * time.Minute,
	// })
	if err != nil {
		testLog.Errorf("pullCidProtocol.Request error: %v", err)
		return
	}

	// go func() {
	// 	timeout := 300 * time.Second
	// 	startTime := time.Now()
	// 	select {
	// 	case pullCidResponse := <-pullCidResponseChan:
	// 		elapsed := time.Since(startTime)
	// 		testLog.Debugf("PullCidClientProtocol->Request: elapsed time: %v", elapsed.Seconds())
	// 		if pullCidResponse == nil {
	// 			testLog.Errorf("PullCidClientProtocol->Request: pullCidResponse is nil")
	// 			return
	// 		}
	// 		switch pullCidResponse.PinStatus {
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

	cfg, err := loadConfig(rootPath)
	if err != nil {
		testLog.Errorf("loadConfig error: %v", err)
		t.Errorf("loadConfig error: %v", err)
		return
	}
	err = initLog()
	if err != nil {
		testLog.Errorf("initLog error: %v", err)
		t.Errorf("initLog error: %v", err)
		return
	}

	srcPrikeyHex, srcPubkeyHex, err := getKeyBySeed(srcSeed)
	if err != nil {
		testLog.Errorf("getKeyBySeed error: %v", err)
		t.Errorf("getKeyBySeed error: %v", err)
		return
	}
	testLog.Infof("src user: seed:%s, prikey:%s, pubkey:%s", srcSeed, srcPrikeyHex, srcPubkeyHex)

	_, err = tvbase.NewTvbase(ctx, cfg, rootPath)
	if err != nil {
		t.Errorf("NewTvbase error: %v", err)
		testLog.Errorf("NewTvbase error: %v", err)
	}

	<-ctx.Done()
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
