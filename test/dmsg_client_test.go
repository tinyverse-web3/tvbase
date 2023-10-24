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
	"github.com/tinyverse-web3/mtv_go_utils/key"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/common/msg"
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
	dmsg.GetMsgClient().SetOnReceiveMsg(onReceiveMsg)
	dmsg.GetMsgClient().SetOnReceiveMsg(onReceiveMsg)
	// publish dest user
	destPubkeyBytes, err := key.ECDSAPublicKeyToProtoBuf(destPubKey)
	if err != nil {
		testLog.Errorf("ECDSAPublicKeyToProtoBuf error: %v", err)
		return
	}
	destPubKeyStr := key.TranslateKeyProtoBufToString(destPubkeyBytes)
	err = dmsg.GetMsgClient().SubscribeDestUser(destPubKeyStr, true)
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

			sendMsgReq, err := dmsg.GetMsgClient().SendMsg(destPubKeyStr, encrypedContent)

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

	err = tb.Start()
	if err != nil {
		testLog.Errorf("initService error: %v", err)
		return nil, nil, err
	}

	err = tb.WaitRendezvous(30 * time.Second)
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

	pubkey := key.TranslateKeyProtoBufToString(srcPubkeyBytes)
	dmsgService, err := CreateDmsgService(tb, pubkey, getSigCallback, true)
	if err != nil {
		return nil, nil, err
	}

	err = dmsgService.Start()
	if err != nil {
		return nil, nil, err
	}

	return tb, dmsgService, nil
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
