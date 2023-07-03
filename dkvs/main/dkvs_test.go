package main

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	ic "github.com/libp2p/go-libp2p/core/crypto"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	dkvs "github.com/tinyverse-web3/tvbase/dkvs"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

func init() {
	// log.SetAllLoggers(log.LevelDebug) //设置所有日志为Debug
	// dkvs.InitAPP(dkvs.LogDebug)
	// dkvs.InitModule(dkvs.DKVS_NAMESPACE, dkvs.LogDebug)
}

func TestDkvs(t *testing.T) {
	//relayAddr := "/ip4/156.251.179.31/tcp/9000/p2p/12D3KooWSYLNGkmanka9QS7kV5CS8kqLZBT2PUwxX7WqL63jnbGx"

	tvbase, err := tvbase.NewTvbase()
	if err != nil {
		t.Fatal(err)
	}

	var tvNode tvCommon.TvBaseService = tvbase
	kv := dkvs.NewDkvs("./", tvNode) //.表示当前路径

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

	tKey := "/" + dkvs.KEY_NS_DAUTH + "/" + hash("dkvs-k001-aa18")
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
	if err != nil {
		t.Fatal(err)
	}

	data = dkvs.GetRecordSignData(tKey, tValue2, pkBytes, issuetime, ttl)
	sigData2, err := priv.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(tKey, tValue2, pkBytes, issuetime, ttl, sigData2)
	if err != nil {
		t.Fatal(err)
	}
	value, _, _, _, sign, err := kv.Get(tKey)
	if err != nil || !bytes.Equal(value, tValue2) || !bytes.Equal(sign, sigData2) {
		t.Fatal(err)
	}

	// sigData3, err := priv.Sign(tValue3)
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// err = dkvs.Put(tKey, tValue3, pkBytes, issuetime, ttl, sigData3)
	// if err != nil {
	// 	t.Fatal(err)
	// }

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
	value, _, _, _, sign, err = kv.Get(tKey)
	if err != nil || !bytes.Equal(value, tValue2) || !bytes.Equal(sign, sigData2) {
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

	// node.Stop()
}

func bytesToHexString(input []byte) string {
	hexString := "0x"
	for _, b := range input {
		hexString += fmt.Sprintf("%02x", b)
	}
	return hexString
}

func TestGun(t *testing.T) {
	//relayAddr := "/ip4/156.251.179.31/tcp/9000/p2p/12D3KooWSYLNGkmanka9QS7kV5CS8kqLZBT2PUwxX7WqL63jnbGx"

	tvbase, err := tvbase.NewTvbase()
	if err != nil {
		t.Fatal(err)
	}
	var mtvNode tvCommon.TvBaseService = tvbase
	kv := dkvs.NewDkvs("./", mtvNode) //.表示当前路径

	seed := "zjMGsKesWSlZnayK" //dkvs.RandString(16)
	gunPrivKey, e := dkvs.GetPriKeyBySeed(seed)
	if e != nil {
		t.Fatal(err)
	}
	fmt.Println("seed ", seed)

	gunPubKey, err := ic.MarshalPublicKey(gunPrivKey.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("public key: ", bytesToHexString(gunPubKey))
	fmt.Println("public key length: ", len(bytesToHexString(gunPubKey)))

	fmt.Println("GUN public key: ", bytesToHexString(dkvs.GetGUNPubKey()))

	// Gun service apply a gun name
	name := dkvs.RandString(8)
	for {
		if !kv.Has(name) {
			break
		}
		name = dkvs.RandString(8)
	}
	key := dkvs.GetGunKey(name)
	fmt.Println("name ", name)

	issueTime := dkvs.TimeNow()
	ttl := dkvs.GetTtlFromDuration(10 * time.Hour)
	issuetime := dkvs.TimeNow()
	data1 := dkvs.GetGunSignData(name, gunPubKey, issueTime, ttl)
	sign1, err := gunPrivKey.Sign(data1)
	if err != nil {
		t.Fatal(err)
	}
	gunvalue := dkvs.EncodeGunValue(name, issueTime, ttl, gunPubKey, sign1, nil)
	newgunvalue := dkvs.EncodeGunValue(name, issueTime, ttl, gunPubKey, sign1, []byte("user data"))

	data := dkvs.GetRecordSignData(key, gunvalue, gunPubKey, issuetime, ttl)
	sigData1, err := gunPrivKey.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put((key), gunvalue, gunPubKey, issuetime, ttl, sigData1)
	if err != nil {
		t.Fatal(err)
	}

	// prevent others to apply gun name directly
	privA, err := dkvs.GetPriKeyBySeed("AAA")
	if err != nil {
		t.Fatal(err)
	}

	pubKeyA, err := ic.MarshalPublicKey(privA.GetPublic())
	if err != nil {
		t.Fatal(err)
	}

	name2 := ("gun" + dkvs.RandString(4))
	key2 := dkvs.GetGunKey(name2)
	tValue1 := []byte("world1")

	data = dkvs.GetRecordSignData(key2, tValue1, pubKeyA, issuetime, ttl)
	sigData1, err = privA.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(key2, tValue1, pubKeyA, issuetime, ttl, sigData1)
	if err == nil {
		t.Fatal(err)
	}

	// gun service transfer a name to A
	err = testTransfer(*kv, name, gunvalue, gunPrivKey, issuetime, ttl, privA)
	if err != nil {
		t.Fatal(err)
	}

	// a GUN record can't be changed to a incorrect format record
	newvalue := []byte("new value")
	data = dkvs.GetRecordSignData(key, newvalue, pubKeyA, issuetime, ttl)
	sigDataA2, err := privA.Sign(data)
	if err != nil {
		t.Fatal(err)
	}

	err = kv.Put(key, newvalue, pubKeyA, issuetime, ttl, sigDataA2)
	if err == nil {
		t.Fatal(err)
	}

	newttl := ttl + 1
	gunvalue2 := dkvs.EncodeGunValue(name, issueTime, newttl, gunPubKey, sign1, nil)
	data = dkvs.GetRecordSignData(key, gunvalue2, pubKeyA, issuetime, newttl)
	sigDataA3, err := privA.Sign(data)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(key, gunvalue, pubKeyA, issuetime, newttl, sigDataA3)
	if err == nil {
		t.Fatal(err)
	}

	// a GUN record can be changed to a correct record
	newData := dkvs.GetRecordSignData(key, newgunvalue, pubKeyA, issuetime, ttl)
	sigDataA4, err := privA.Sign(newData)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(key, newgunvalue, pubKeyA, issuetime, ttl, sigDataA4)
	if err != nil {
		t.Fatal(err)
	}

	// the name can put another pubkey
	name22 := "/" + name + "/" + bytesToHexString(pubKeyA)
	tValue22 := []byte("world1")
	data22 := dkvs.GetRecordSignData(name22, tValue22, pubKeyA, issuetime, ttl)
	sigData22, err := privA.Sign(data22)
	if err != nil {
		t.Fatal(err)
	}
	err = kv.Put(name22, tValue22, pubKeyA, issuetime, ttl, sigData22)
	if err != nil {
		t.Fatal(err)
	}

	// a gun record can be transfered.
	privB, err := dkvs.GetPriKeyBySeed("BBB")
	if err != nil {
		t.Fatal(err)
	}

	// transfer fail，只能手工测试，条件太难配置
	// err = testTransferRestore(*kv, name, gunvalue, privA, issuetime, ttl, privB)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// transfer succ
	err = testTransfer(*kv, name, newgunvalue, privA, issuetime, ttl, privB)
	if err != nil {
		t.Fatal(err)
	}

	privC, err := dkvs.GetPriKeyBySeed("CCC")
	if err != nil {
		t.Fatal(err)
	}

	// A can't transfer a non-owned name to B
	err = testTransfer(*kv, name, gunvalue, privA, issuetime, ttl, privC)
	if err == nil {
		t.Fatal(err)
	}

}

func testTransfer(kv dkvs.Dkvs, name string, gunvalue []byte, privk1 ic.PrivKey, issuetime uint64, ttl uint64, privk2 ic.PrivKey) error {
	pubkey1, _ := ic.MarshalPublicKey(privk1.GetPublic())
	pubkey2, _ := ic.MarshalPublicKey(privk2.GetPublic())

	// B sign the record
	key := dkvs.GetGunKey(name)
	data := dkvs.GetRecordSignData(key, gunvalue, pubkey2, issuetime, ttl)
	sign2, err := privk2.Sign(data)
	if err != nil {
		return (err)
	}

	// owner sign a transfer record
	signData1 := dkvs.GetRecordSignData(key, pubkey2, pubkey1, issuetime, ttl)
	if signData1 == nil {
		return (err)
	}
	sign1, err := privk1.Sign(signData1)
	if err != nil {
		return (err)
	}

	// then A tranfer a name to B
	err = kv.TransferKey(key, pubkey1, sign1, gunvalue, pubkey2, issuetime, ttl, sign2)
	if err != nil {
		return (err)
	}

	// check
	value3, key3, issuetime3, ttl3, sign3, err3 := kv.Get(key)
	if err3 != nil {
		return (err3)
	}
	err = errors.New("new error")
	if !bytes.Equal(value3, gunvalue) {
		return (err)
	}
	if !bytes.Equal(key3, pubkey2) {
		return (err)
	}
	if !bytes.Equal(sign3, sign2) {
		return (err)
	}
	if issuetime3 != issuetime {
		return err
	}
	if ttl3 != ttl {
		return (err)
	}

	return nil
}

func testTransferRestore(kv dkvs.Dkvs, name string, gunvalue []byte, privk1 ic.PrivKey, issuetime uint64, ttl uint64, privk2 ic.PrivKey) error {
	pubkey1, _ := ic.MarshalPublicKey(privk1.GetPublic())
	pubkey2, _ := ic.MarshalPublicKey(privk2.GetPublic())

	// B sign the record
	data := dkvs.GetRecordSignData(name, gunvalue, pubkey2, issuetime, ttl)
	sign2, err := privk2.Sign(data)
	if err != nil {
		return (err)
	}

	// owner sign a transfer record
	signData1 := dkvs.GetRecordSignData(name, pubkey2, pubkey1, issuetime, ttl)
	if signData1 == nil {
		return (err)
	}
	sign1, err := privk1.Sign(signData1)
	if err != nil {
		return (err)
	}

	value3, key3, issuetime3, ttl3, sign3, err3 := kv.Get(name)
	if err3 != nil {
		return (err)
	}

	// then A tranfer a name to B
	err = kv.TransferKey(name, pubkey1, sign1, gunvalue, pubkey2, issuetime, ttl, sign2)
	if err == nil {
		return (err)
	}

	// check
	err = errors.New("new error")
	value4, key4, issue4, ttl4, sign4, err4 := kv.Get(name)
	if err4 != nil {
		return (err)
	}
	if !bytes.Equal(value3, value4) {
		return (err)
	}
	if !bytes.Equal(key3, key4) {
		return (err)
	}
	if !bytes.Equal(sign3, sign4) {
		return (err)
	}
	if issuetime3 != issue4 {
		return (err)
	}
	if ttl3 != ttl4 {
		return (err)
	}

	return nil
}

func hash(key string) (hashKey string) {
	shaHash := sha512.Sum384([]byte(key))
	hashKey = hex.EncodeToString(shaHash[:])
	return
}

func TestLDB(t *testing.T) {
	//relayAddr := "/ip4/156.251.179.31/tcp/9000/p2p/12D3KooWSYLNGkmanka9QS7kV5CS8kqLZBT2PUwxX7WqL63jnbGx"

	node, err := tvbase.NewTvbase()
	if err != nil {
		t.Fatal(err)
	}

	var mtvNode tvCommon.TvBaseService
	mtvNode = node
	dkvs.NewDkvs("./", mtvNode) //.表示当前路径

	dkvs.TestSyncDB()
}
