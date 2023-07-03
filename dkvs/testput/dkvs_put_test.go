package testput

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	ic "github.com/libp2p/go-libp2p/core/crypto"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
	dkvs "github.com/tinyverse-web3/tvbase/dkvs"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

func init() {
	// log.SetAllLoggers(log.LevelDebug) //设置所有日志为Debug
	// dkvs.InitAPP(dkvs.LogDebug)
	// dkvs.InitModule(dkvs.DKVS_NAMESPACE, dkvs.LogDebug)

	err := tvUtil.InitConfig()
	if err != nil {
		return
	}
	err = tvUtil.InitLog()
	if err != nil {
		return
	}
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

func TestDkvsPutKeyToOtherNode(t *testing.T) {
	//relayAddr := "/ip4/156.251.179.31/tcp/9000/p2p/12D3KooWSYLNGkmanka9QS7kV5CS8kqLZBT2PUwxX7WqL63jnbGx"

	tvbase, err := tvbase.NewTvbase()
	if err != nil {
		t.Fatal(err)
	}
	var tvNode tvCommon.TvBaseService = tvbase
	kv := dkvs.NewDkvs("./", tvNode) //.表示当前路径
	seed := "oIBBgepoPyhdJTYB"       //dkvs.RandString(16)
	priv, err := dkvs.GetPriKeyBySeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	pubKey := priv.GetPublic()
	pkBytes, err := ic.MarshalPublicKey(pubKey)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("seed: ", seed)
	fmt.Println("pubkey: ", bytesToHexString(pkBytes))

	tKey := "/" + dkvs.KEY_NS_DAUTH + "/" + hash("dkvs-pk001-0021")
	tValue1 := []byte("world1")
	tValue2 := []byte("mtv2")
	tValue3 := []byte("mtv3")
	tValue4 := []byte("mtv4")
	ttl := dkvs.GetTtlFromDuration(time.Hour)
	issuetime := dkvs.TimeNow()

	data := dkvs.GetRecordSignData(tKey, tValue1, pkBytes, issuetime, ttl)
	sigData1, err := priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue1, pkBytes, issuetime, ttl, sigData1)
	select {}
	// select {
	// case <-time.After(30 * time.Second):
	// 	fmt.Println("Timeout occurred")
	// }
	if err != nil {
		t.Fatal(err)
	}
	value, _, _, _, sign, err := kv.Get(tKey)
	if err != nil || !bytes.Equal(value, tValue1) || !bytes.Equal(sign, sigData1) {
		t.Fatal(err)
	}

	data = dkvs.GetRecordSignData(tKey, tValue2, pkBytes, issuetime, ttl)
	sigData2, err := priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue2, pkBytes, issuetime, ttl, sigData2)
	select {
	case <-time.After(20 * time.Second):
		fmt.Println("Timeout occurred")
	}
	if err != nil {
		t.Fatal(err)
	}
	value, _, _, _, sign, err = kv.Get(tKey)
	if err != nil || !bytes.Equal(value, tValue2) || !bytes.Equal(sign, sigData2) {
		t.Fatal(err)
	}

	priv2, err := dkvs.GetPriKeyBySeed("mtv3")
	if err != nil {
		t.Fatal(err)
	}
	pubKey2 := priv2.GetPublic()
	pkBytes2, err := ic.MarshalPublicKey(pubKey2)
	if err != nil {
		t.Fatal(err)
	}

	data = dkvs.GetRecordSignData(tKey, tValue3, pkBytes2, issuetime, ttl)
	sigData3, err := priv2.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue3, pkBytes2, issuetime, ttl, sigData3)
	if err != nil {
		fmt.Println(err)
	} else {
		t.Fatal(err)
	}
	select {
	case <-time.After(30 * time.Second):
		fmt.Println("Timeout occurred")
	}
	value, _, _, _, sign, err = kv.Get(tKey)
	if err != nil || bytes.Equal(value, tValue3) || bytes.Equal(sign, sigData3) {
		t.Fatal(err)
	}

	data = dkvs.GetRecordSignData(tKey, tValue4, pkBytes, issuetime, ttl)
	sigData4, err := priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue4, pkBytes, issuetime, ttl, sigData4)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-time.After(30 * time.Second):
		fmt.Println("Timeout occurred")
	}
	value, _, _, _, sign, err = kv.Get(tKey)
	if err != nil || !bytes.Equal(value, tValue4) || !bytes.Equal(sign, sigData4) {
		t.Fatal(err)
	}

	// use a pubkey as key
	tKey = "/" + dkvs.KEY_NS_DAUTH + "/" + bytesToHexString(pkBytes)
	data = dkvs.GetRecordSignData(tKey, tValue4, pkBytes2, issuetime, ttl)
	sigData5, err := priv2.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue4, pkBytes2, issuetime, ttl, sigData5)
	if err == nil {
		t.Fatal(err)
	}

	data = dkvs.GetRecordSignData(tKey, tValue4, pkBytes, issuetime, ttl)
	sigData6, err := priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue4, pkBytes, issuetime, ttl, sigData6)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-time.After(30 * time.Second):
		fmt.Println("Timeout occurred")
	}

	value, _, _, _, sign, err = kv.Get(tKey)
	if err != nil || !bytes.Equal(value, tValue4) || !bytes.Equal(sign, sigData6) {
		t.Fatal(err)
	}

	tvbase.Stop()
}

func TestUnsyncedDb(t *testing.T) {
	tvbase, err := tvbase.NewTvbase()
	if err != nil {
		t.Fatal(err)
	}
	var mtvNode tvCommon.TvBaseService = tvbase
	kv := dkvs.NewDkvs("./", mtvNode) //.表示当前路径

	seed := "oIBBgepoPyhdJTYB" //dkvs.RandString(16)
	priv, err := dkvs.GetPriKeyBySeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	pubKey := priv.GetPublic()
	pkBytes, err := ic.MarshalPublicKey(pubKey)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("seed: ", seed)
	fmt.Println("pubkey: ", bytesToHexString(pkBytes))

	tKey := "/" + dkvs.KEY_NS_DAUTH + "/" + hash("dkvs-usync001")
	tValue1 := []byte("world1")
	ttl := dkvs.GetTtlFromDuration(time.Hour)
	issuetime := dkvs.TimeNow()

	data := dkvs.GetRecordSignData(tKey, tValue1, pkBytes, issuetime, ttl)
	sigData1, err := priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue1, pkBytes, issuetime, ttl, sigData1)
	if err != nil {
		t.Fatal(err)
	}

	tKey = "/" + dkvs.KEY_NS_DAUTH + "/" + hash("dkvs-usync002")
	data = dkvs.GetRecordSignData(tKey, tValue1, pkBytes, issuetime, ttl)
	sigData1, err = priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue1, pkBytes, issuetime, ttl, sigData1)
	if err != nil {
		t.Fatal(err)
	}

	tKey = "/" + dkvs.KEY_NS_DAUTH + "/" + hash("dkvs-usync003")
	data = dkvs.GetRecordSignData(tKey, tValue1, pkBytes, issuetime, ttl)
	sigData1, err = priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue1, pkBytes, issuetime, ttl, sigData1)
	if err != nil {
		t.Fatal(err)
	}

	tKey = "/" + dkvs.KEY_NS_DAUTH + "/" + hash("dkvs-usync004")
	data = dkvs.GetRecordSignData(tKey, tValue1, pkBytes, issuetime, ttl)
	sigData1, err = priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue1, pkBytes, issuetime, ttl, sigData1)
	if err != nil {
		t.Fatal(err)
	}

	tKey = "/" + dkvs.KEY_NS_DAUTH + "/" + hash("dkvs-usync005")
	data = dkvs.GetRecordSignData(tKey, tValue1, pkBytes, issuetime, ttl)
	sigData1, err = priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue1, pkBytes, issuetime, ttl, sigData1)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-time.After(60 * time.Second):
		fmt.Println("Timeout occurred")
	}
	tvbase.Stop()

}

func TestPutUnsyncedKeyToOtherNode(t *testing.T) {
	tvbase, err := tvbase.NewTvbase()
	if err != nil {
		t.Fatal(err)
	}
	var mtvNode tvCommon.TvBaseService = tvbase
	kv := dkvs.NewDkvs("./", mtvNode) //.表示当前路径

	seed := "oIBBgepoPyhdJTYB" //dkvs.RandString(16)
	priv, err := dkvs.GetPriKeyBySeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	pubKey := priv.GetPublic()
	pkBytes, err := ic.MarshalPublicKey(pubKey)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("seed: ", seed)
	fmt.Println("pubkey: ", bytesToHexString(pkBytes))

	tKey := "/" + dkvs.KEY_NS_DAUTH + "/" + hash("dkvs-usync16-001")
	tValue1 := []byte("world2")
	ttl := dkvs.GetTtlFromDuration(time.Hour)
	issuetime := dkvs.TimeNow()

	data := dkvs.GetRecordSignData(tKey, tValue1, pkBytes, issuetime, ttl)
	sigData1, err := priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue1, pkBytes, issuetime, ttl, sigData1)
	if err != nil {
		t.Fatal(err)
	}

	tKey = "/" + dkvs.KEY_NS_DAUTH + "/" + hash("dkvs-usync16-002")
	data = dkvs.GetRecordSignData(tKey, tValue1, pkBytes, issuetime, ttl)
	sigData1, err = priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue1, pkBytes, issuetime, ttl, sigData1)
	if err != nil {
		t.Fatal(err)
	}

	tKey = "/" + dkvs.KEY_NS_DAUTH + "/" + hash("dkvs-usync16-003")
	data = dkvs.GetRecordSignData(tKey, tValue1, pkBytes, issuetime, ttl)
	sigData1, err = priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue1, pkBytes, issuetime, ttl, sigData1)
	if err != nil {
		t.Fatal(err)
	}

	select {}

	// node.Stop()

}
