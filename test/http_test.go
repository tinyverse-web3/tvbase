package test

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/define"
	"github.com/tinyverse-web3/tvbase/dkvs"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

func init() {
}

func TestHttpServer(t *testing.T) {
	ctx := context.Background()
	cfg := config.NewDefaultTvbaseConfig()
	cfg.InitMode(config.ServiceMode)
	tvbase, err := tvbase.NewTvbase(ctx, cfg, "./testdata")
	if err != nil {
		t.Fatal(err)
	}
	putSomeValue(tvbase)
	//corehttp.InitHttpServer(tvbase) InitHttpServer已经集成到Tvbase中去了
	select {}
	//通过postman发送请求来测试http api
}

func putSomeValue(tvbase define.TvBaseService) error {
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

	tKey1 := "/" + dkvs.PUBSERVICE_DAUTH + "/" + hash("dkvs-tk001-016")
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

	tKey2 := "/" + dkvs.PUBSERVICE_DAUTH + "/" + hash("dkvs-tk002-016")
	data = dkvs.GetRecordSignData(tKey2, tValue1, pkBytes, issuetime, ttl)
	sigData1, err = priv.Sign(data)
	if err != nil {
		return err
	}
	err = kv.Put(tKey2, tValue1, pkBytes, issuetime, ttl, sigData1)
	if err != nil {
		return err
	}

	tKey3 := "/" + dkvs.PUBSERVICE_DAUTH + "/" + hash("dkvs-tk003-016")
	data = dkvs.GetRecordSignData(tKey3, tValue1, pkBytes, issuetime, ttl)
	sigData1, err = priv.Sign(data)
	if err != nil {
		return err
	}
	err = kv.Put(tKey3, tValue1, pkBytes, issuetime, ttl, sigData1)
	if err != nil {
		return err
	}
	xyz, err := kv.GetRecord(tKey3)
	if err != nil {
		dkvs.Logger.Debugf("{tKey3: %v, err: %s}", tKey3, err.Error())
		return err
	}
	dkvs.Logger.Debugf("{tKey3: %v, value: %v}", tKey3, xyz)
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
