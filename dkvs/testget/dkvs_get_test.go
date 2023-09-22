package testget

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"testing"

	ic "github.com/libp2p/go-libp2p/core/crypto"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/define"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
	dkvs "github.com/tinyverse-web3/tvbase/dkvs"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

func init() {
	logCfg := map[string]string{
		"tvbase":         "debug",
		"dkvs":           "debug",
		"dmsg":           "debug",
		"customProtocol": "debug",
		"tvnode":         "debug",
		"tvipfs":         "debug",
		"core_http":      "debug",
	}
	err := tvUtil.SetLogModule(logCfg)
	if err != nil {
		fmt.Printf("init error: %v", err)
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

func TestDkvsGetKeyFromOtherNode(t *testing.T) {
	//relayAddr := "/ip4/156.251.179.31/tcp/9000/p2p/12D3KooWSYLNGkmanka9QS7kV5CS8kqLZBT2PUwxX7WqL63jnbGx"

	ctx := context.Background()
	cfg := config.NewDefaultTvbaseConfig()
	cfg.InitMode(define.LightMode)
	tvbase, err := tvbase.NewTvbase(ctx, cfg, "./")
	if err != nil {
		t.Fatal(err)
	}

	kv := tvbase.GetDkvsService() //.表示当前路径
	// select {
	// case <-time.After(30 * time.Second):
	// 	fmt.Println("Timeout occurred")
	// }
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

	tKey := "/" + dkvs.PUBSERVICE_DAUTH + "/" + hash("dkvs-pk001-0022")
	tValue1 := []byte("world1")
	tValue2 := []byte("mtv2")
	tValue3 := []byte("mtv3")
	tValue4 := []byte("mtv4")
	// select {
	// case <-time.After(30 * time.Second):
	// 	fmt.Println("Timeout occurred")
	// }
	value, _, _, _, _, err := kv.Get(tKey)
	if err != nil || !bytes.Equal(value, tValue1) {
		t.Fatal(err)
	}

	value, _, _, _, _, err = kv.Get(tKey)
	if err != nil || !bytes.Equal(value, tValue2) {
		t.Fatal(err)
	}

	value, _, _, _, _, err = kv.Get(tKey)
	if err != nil || bytes.Equal(value, tValue3) {
		t.Fatal(err)
	}

	value, _, _, _, _, err = kv.Get(tKey)
	if err != nil || !bytes.Equal(value, tValue4) {
		t.Fatal(err)
	}

	// use a pubkey as key
	tKey = "/" + dkvs.PUBSERVICE_DAUTH + "/" + bytesToHexString(pkBytes)

	value, _, _, _, _, err = kv.Get(tKey)
	if err != nil || !bytes.Equal(value, tValue4) {
		t.Fatal(err)
	}

	tvbase.Stop()
}

func TestGetUnsyncedKeyFromOtherNode(t *testing.T) {
	ctx := context.Background()
	cfg := config.NewDefaultTvbaseConfig()
	cfg.InitMode(define.LightMode)
	tvbase, err := tvbase.NewTvbase(ctx, cfg, "./")
	if err != nil {
		t.Fatal(err)
	}
	var tvBase tvCommon.TvBaseService = tvbase
	kv := dkvs.NewDkvs(tvBase) //.表示当前路径

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

	tKey := "/" + dkvs.PUBSERVICE_DAUTH + "/" + hash("dkvs-usync7-001")
	tValue1 := []byte("world2")

	value, _, _, _, _, err := kv.Get(tKey)
	if err != nil || !bytes.Equal(value, tValue1) {
		t.Fatal(err)
	}

	tKey = "/" + dkvs.PUBSERVICE_DAUTH + "/" + hash("dkvs-usync7-002")
	value, _, _, _, _, err = kv.Get(tKey)
	if err != nil || !bytes.Equal(value, tValue1) {
		t.Fatal(err)
	}

	tKey = "/" + dkvs.PUBSERVICE_DAUTH + "/" + hash("dkvs-usync7-003")
	value, _, _, _, _, err = kv.Get(tKey)
	if err != nil || !bytes.Equal(value, tValue1) {
		t.Fatal(err)
	}
	tvbase.Stop()

}
