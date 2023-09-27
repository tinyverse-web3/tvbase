package corehttp

import (
	"context"
	"encoding/hex"
	"strconv"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	manet "github.com/multiformats/go-multiaddr/net"
	mtvCrypto "github.com/tinyverse-web3/mtv_go_utils/crypto"
	keyUtils "github.com/tinyverse-web3/mtv_go_utils/key"
	define "github.com/tinyverse-web3/tvbase/common/define"
	tvPb "github.com/tinyverse-web3/tvbase/common/protocol/pb"
)

type DemoKey struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	AesKey     string `json:"aes_key"`
}

type DemoEncryptedValue struct {
	EncryptedValue string `json:"encrypted_value"`
	SignData       string `json:"sign_data"`
	Ttl            string `json:"ttl"`
	IssueTime      string `json:"issue_time"`
}
type PutDemoKeyResult struct {
	Result string `json:"result"`
}

type GetDemoKeyResult struct {
	Key              string `json:"key"`
	PutTime          string `json:"put_time"`
	Validity         string `json:"validity"`
	PubKey           string `json:"pub_key"`
	EncryptedValue   string `json:"encrypted_value"`
	UnencryptedValue string `json:"unencrypted_value"`
}

// 返回所有bootstrap节点及已经连接过的服务节点
func getConnetedServerNode(t define.TvBaseService) []Node {
	host := t.GetHost()
	peerstore := host.Peerstore()
	peers := host.Network().Peers()
	nodeInfoService := t.GetNodeInfoService()
	var nodeList []Node
	var mutex sync.Mutex
	var wg sync.WaitGroup
	var servicePeerIds []peer.ID
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, p := range peers {
		pId := p
		wg.Add(1) // 增加WaitGroup的计数
		go func() {
			defer wg.Done() // 当goroutine完成时减少WaitGroup的计数
			result := nodeInfoService.Request(ctx, pId)
			if result == nil {
				Logger.Debugf("getConnetedServerNode->try get peer info: %v, result is nil", pId.Pretty())
				return
			}
			if result.Error != nil {
				Logger.Debugf("getConnetedServerNode: try get peer info: %v happen error, result: %v", pId.Pretty(), result)
				return
			}

			if result.NodeInfo == nil {
				Logger.Warnf("getConnetedServerNode: result node info is nil: pereID %s", pId.Pretty())
				return
			}
			switch result.NodeInfo.NodeType {
			case tvPb.NodeType_Light:
			case tvPb.NodeType_Full:
				// 使用互斥锁保护共享变量的修改
				mutex.Lock()
				servicePeerIds = append(servicePeerIds, pId)
				mutex.Unlock()
				Logger.Debugf("getConnetedServerNode--->Successfully connected to the servcie node: {id: %s}", pId.Pretty())
			}
		}()
	}
	// 等待所有goroutine完成
	wg.Wait()

	for _, peerId := range servicePeerIds {
		var node Node
		addrInfo := peerstore.PeerInfo(peerId)
		publicIp := getPublicIpFromAddr(addrInfo)
		node.NodeId = peerId.Pretty()
		node.Addrs = addrInfo.String()
		node.PublicIp = publicIp
		nodeList = append(nodeList, node)
	}
	return nodeList
}

func getPublicIpFromAddr(addInfo peer.AddrInfo) string {
	addrs := addInfo.Addrs
	publicIp := ""
	for _, addr := range addrs {
		ip, err := manet.ToIP(addr)
		if err == nil {
			publicIp = ip.String()
			break
		}
	}
	return publicIp
}

func getDemoKey(seed string) DemoKey {
	var demoKey = new(DemoKey)
	demoKey.PrivateKey = ""
	demoKey.PublicKey = ""
	priv, pubKey, err := keyUtils.GetPairKeyBySeed(seed)
	if err != nil {
		demoKey.PrivateKey = err.Error()
		demoKey.PublicKey = err.Error()
		return *demoKey
	}
	privateKeyStr, err := keyUtils.TranslatePrivateKeyToString(priv)
	if err != nil {
		demoKey.PrivateKey = err.Error()
		demoKey.PublicKey = ""
		return *demoKey
	}
	demoKey.PrivateKey = privateKeyStr
	publicKeyStr, err := keyUtils.TranslatePublicKeyToString(pubKey)
	if err != nil {
		demoKey.PrivateKey = privateKeyStr
		demoKey.PublicKey = err.Error()
		return *demoKey
	}
	demoKey.PublicKey = publicKeyStr
	demoKey.AesKey = generateAESKey(privateKeyStr, seed)

	return *demoKey
}

func getEncryptedDemoValue(privateKey, aesKey, dkvsKey, dkvsValue string) DemoEncryptedValue {
	var demoEncryptedValue = new(DemoEncryptedValue)
	demoEncryptedValue.EncryptedValue = ""
	encryptedValue := mtvCrypto.EncryptAES([]byte(dkvsValue), []byte(aesKey))
	demoEncryptedValue.EncryptedValue = keyUtils.TranslateKeyProtoBufToString(encryptedValue)
	ttl := getTtlFromDuration(time.Hour)
	issuetime := timeNow()
	demoEncryptedValue.Ttl = strconv.FormatUint(ttl, 10)
	demoEncryptedValue.IssueTime = strconv.FormatUint(issuetime, 10)

	priv, err := keyUtils.TranslatePrivateKeyStringToCrypt(privateKey)
	if err != nil {
		demoEncryptedValue.SignData = err.Error()
		return *demoEncryptedValue
	}
	pubKey := priv.GetPublic()
	pkBytes, err := keyUtils.TranslatePublicKeyToProtoBuf(pubKey)
	if err != nil {
		demoEncryptedValue.SignData = err.Error()
		return *demoEncryptedValue
	}
	signDataStr := getRecordSignData(dkvsKey, []byte(dkvsValue), pkBytes, issuetime, ttl)
	demoEncryptedValue.SignData = keyUtils.TranslateKeyProtoBufToString(signDataStr)
	return *demoEncryptedValue
}

func generateAESKey(privateKey, seed string) string {
	seedHash := hash512([]byte(seed))
	keyHash := hash512([]byte(privateKey))

	subStr := seedHash[0:8] + keyHash[0:8]
	passwordHash := hash128([]byte(subStr))
	return passwordHash
}

func pubDemoKey(t define.TvBaseService, privateKey, dkvsKey, dkvsValue, ttlStr, issueTimeStr string) PutDemoKeyResult {
	var result = new(PutDemoKeyResult)
	result.Result = ""
	dkvs := t.GetDkvsService()

	ttl, err := strconv.ParseUint(ttlStr, 10, 64)
	if err != nil {
		result.Result = err.Error()
		return *result
	}
	issuetime, err := strconv.ParseUint(issueTimeStr, 10, 64)
	if err != nil {
		result.Result = err.Error()
		return *result
	}

	priv, err := keyUtils.TranslatePrivateKeyStringToCrypt(privateKey)
	if err != nil {
		result.Result = err.Error()
		return *result
	}
	pubKey := priv.GetPublic()
	pkBytes, err := keyUtils.TranslatePublicKeyToProtoBuf(pubKey)
	if err != nil {
		result.Result = err.Error()
		return *result
	}
	signDataStr := getRecordSignData(dkvsKey, []byte(dkvsValue), pkBytes, issuetime, ttl)
	signData, err := priv.Sign(signDataStr)
	if err != nil {
		result.Result = err.Error()
		return *result
	}
	err = dkvs.Put(dkvsKey, []byte(dkvsValue), pkBytes, issuetime, ttl, signData)
	if err != nil {
		result.Result = err.Error()
		return *result
	}
	result.Result = "Success Put to DKVS network"
	return *result
}

func getDemoKeyValue(t define.TvBaseService, aesKey, dkvsKey string) GetDemoKeyResult {
	var result = new(GetDemoKeyResult)
	result.Key = dkvsKey
	dkvs := t.GetDkvsService()
	dkvsRec, err := dkvs.GetRecord(dkvsKey)
	if err != nil {
		result.EncryptedValue = err.Error()
		return *result
	}
	byteValue, err := hex.DecodeString(string(dkvsRec.Value))
	if err != nil {
		result.EncryptedValue = err.Error()
		return *result
	}
	result.EncryptedValue = string(dkvsRec.Value)
	result.PutTime = strconv.FormatUint(dkvsRec.Seq, 10)
	result.Validity = strconv.FormatUint(dkvsRec.Validity, 10)
	result.PubKey = hex.EncodeToString(dkvsRec.PubKey)
	result.UnencryptedValue = string(mtvCrypto.DecryptAES(byteValue, []byte(aesKey)))
	return *result
}
