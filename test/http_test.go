package test

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	ic "github.com/libp2p/go-libp2p/core/crypto"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
	"github.com/tinyverse-web3/tvbase/corehttp"
	"github.com/tinyverse-web3/tvbase/dkvs"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

func init() {
	nodeConfig, err := tvUtil.LoadNodeConfig()
	if err != nil {
		fmt.Printf("init error: %v", err)
		return
	}
	err = tvUtil.SetLogModule(nodeConfig.Log.ModuleLevels)
	if err != nil {
		fmt.Printf("init error: %v", err)
		return
	}
	corehttp.InitLog()
}

func TestHttpServer(t *testing.T) {
	tvbase, err := tvbase.NewTvbase("./testdata") //如果不传入任何参数，默认数据存储路径是当前路径下
	if err != nil {
		t.Fatal(err)
	}
	putSomeValue(tvbase)
	// corehttp.InitHttpServer(tvbase)
	select {}
}

func putSomeValue(tvbase tvCommon.TvBaseService) error {
	kv := tvbase.GetDkvsService() //.表示当前路径
	seed := "oIBBgepoPyhdJTYB"    //dkvs.RandString(16)
	priv, err := dkvs.GetPriKeyBySeed(seed)
	if err != nil {
		return err
	}
	pubKey := priv.GetPublic()
	pkBytes, err := ic.MarshalPublicKey(pubKey)
	if err != nil {
		return err
	}

	fmt.Println("seed: ", seed)
	fmt.Println("pubkey: ", bytesToHexString(pkBytes))

	tKey1 := "/" + dkvs.KEY_NS_DAUTH + "/" + hash("dkvs-tk001-000")
	tValue1 := []byte("world1")
	ttl := dkvs.GetTtlFromDuration(time.Hour)
	issuetime := dkvs.TimeNow()
	data := dkvs.GetRecordSignData(tKey1, tValue1, pkBytes, issuetime, ttl)
	sigData1, err := priv.Sign(data)
	if err != nil {
		return err
	}
	err = kv.Put(tKey1, tValue1, pkBytes, issuetime, ttl, sigData1)
	if err != nil {
		return err
	}

	tKey2 := "/" + dkvs.KEY_NS_DAUTH + "/" + hash("dkvs-tk002-000")
	data = dkvs.GetRecordSignData(tKey2, tValue1, pkBytes, issuetime, ttl)
	sigData1, err = priv.Sign(data)
	if err != nil {
		return err
	}
	err = kv.Put(tKey2, tValue1, pkBytes, issuetime, ttl, sigData1)
	if err != nil {
		return err
	}

	tKey3 := "/" + dkvs.KEY_NS_DAUTH + "/" + hash("dkvs-tk003-000")
	data = dkvs.GetRecordSignData(tKey3, tValue1, pkBytes, issuetime, ttl)
	sigData1, err = priv.Sign(data)
	if err != nil {
		return err
	}
	err = kv.Put(tKey3, tValue1, pkBytes, issuetime, ttl, sigData1)
	if err != nil {
		return err
	}
	return nil
}

func hash(key string) (hashKey string) {
	shaHash := sha512.Sum384([]byte(key))
	hashKey = hex.EncodeToString(shaHash[:])
	return
}

func bytesToHexString(input []byte) string {
	hexString := "0x"
	for _, b := range input {
		hexString += fmt.Sprintf("%02x", b)
	}
	return hexString
}
