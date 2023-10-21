package corehttp

import (
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/gogo/protobuf/proto"
	ds "github.com/ipfs/go-datastore"
	recpb "github.com/libp2p/go-libp2p-record/pb"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-base32"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/tinyverse-web3/tvbase/dkvs"
	dkvs_pb "github.com/tinyverse-web3/tvbase/dkvs/pb"
)

const (
	defaultPathName = ".tvnode"
	defaultPathRoot = "./" + defaultPathName
)

func dsKeyDcode(s string) ([]byte, error) {
	return base32.RawStdEncoding.DecodeString(s)
}

func isPrivateIP(ip net.IP) bool {
	// 私网地址前缀
	privateIPPrefixes := []string{
		"10.",
		"172.16.",
		"172.17.",
		"172.18.",
		"172.19.",
		"172.20.",
		"172.21.",
		"172.22.",
		"172.23.",
		"172.24.",
		"172.25.",
		"172.26.",
		"172.27.",
		"172.28.",
		"172.29.",
		"172.30.",
		"172.31.",
		"192.168.",
	}

	for _, prefix := range privateIPPrefixes {
		if ip.To4() == nil {
			continue
		}
		// Logger.Debugf("ip: %v", ip.String())
		if ip.String()[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

func isPrivateNode(addInfo peer.AddrInfo) (bool, string) {
	addrs := addInfo.Addrs
	isPrivate := false
	publicIp := ""
	for _, addr := range addrs {
		ip, err := manet.ToIP(addr)
		if err == nil && isPrivateIP(ip) {
			isPrivate = true
			break
		}
		publicIp = ip.String()
	}
	return isPrivate, publicIp
}

func getTvBaseDataDir() string {
	currentUser, err := user.Current()
	relPath := defaultPathRoot
	if err == nil {
		relPath = currentUser.HomeDir + string(os.PathSeparator) + defaultPathName
	}
	absPath, _ := filepath.Abs(relPath)
	_, err = os.Stat(absPath)
	if err != nil || os.IsNotExist(err) {
		Logger.Debugf("Path is not exist {path: %s, err: %s}", absPath, err.Error())
		absPath, err = os.Getwd()
		if err != nil {
			Logger.Debugf("os.Getwd() is not exist {err: %s}", err.Error())
			absPath, _ = filepath.Abs("./")
		}
	}
	Logger.Debugf("current path: {path: %s}", absPath)
	return absPath
}

func getDirSizeAndFileCount(path string) (int64, int64) {
	var size int64
	var count int64 = 0

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
			count += 1
		}
		return err
	})
	if err != nil {
		return 0, 0
	}
	return size, count
}

func mkDsKey(s string) ds.Key {
	return ds.NewKey(base32.RawStdEncoding.EncodeToString([]byte(s)))
}

func getValueFromLibp2pRec(libP2pVal []byte) (string, error) {
	lbp2pRec := new(recpb.Record)
	err := proto.Unmarshal(libP2pVal, lbp2pRec)
	if err != nil {
		Logger.Errorf("getValueFromRec---> proto.Unmarshal(result.Value, lbp2pRec) failed: %v", err)
		return "", err
	}
	return getValueFromDkvsRec(lbp2pRec.Value)
}

func getValueFromDkvsRec(dkvsVal []byte) (string, error) {
	dkvsRec := new(dkvs_pb.DkvsRecord)
	if err := proto.Unmarshal(dkvsVal, dkvsRec); err != nil {
		Logger.Errorf("getValueFromDkvsRec---> proto.Unmarshal(rec.Value, dkvsRec) failed: %v", err)
		return "", err
	}
	return string(dkvsRec.Value), nil
}

func getKVDetailsFromLibp2pRec(libp2pKey string, libP2pVal []byte) (*DkvsKV, error) {
	lbp2pRec := new(recpb.Record)
	err := proto.Unmarshal(libP2pVal, lbp2pRec)
	if err != nil {
		Logger.Errorf("get value from libp2p record---> proto.Unmarshal(libP2pVal lbp2pRec) failed: %v", err)
		return nil, err
	}
	dkvsRec := new(dkvs_pb.DkvsRecord)
	if err := proto.Unmarshal(lbp2pRec.Value, dkvsRec); err != nil {
		Logger.Errorf("get value from dkvs recrod---> proto.Unmarshal(lbp2pRec.Value, dkvsRec) failed: %v", err)
		return nil, err
	}
	var kv DkvsKV
	kv.Key = string(dkvs.RemovePrefix(libp2pKey))
	kv.Value = string(dkvsRec.Value)
	kv.PutTime = formatUnixTime(dkvsRec.Seq)
	kv.Validity = formatUnixTime(dkvsRec.Validity)
	kv.PubKey = hex.EncodeToString(dkvsRec.PubKey)
	return &kv, nil
}

func getKVDetailsFromDkvsRec(libp2pKey string, dkvsRec *dkvs_pb.DkvsRecord) (*DkvsKV, error) {
	var kv DkvsKV
	kv.Key = string(dkvs.RemovePrefix(libp2pKey))
	kv.Value = string(dkvsRec.Value)
	kv.PutTime = formatUnixTime(dkvsRec.Seq)
	kv.Validity = formatUnixTime(dkvsRec.Validity)
	kv.PubKey = hex.EncodeToString(dkvsRec.PubKey)
	return &kv, nil
}

func formatUnixTime(unixTime uint64) string {
	// convert Unix timestamp() to time.Time type
	timeObj := time.Unix(int64(unixTime)/1000, int64(unixTime)%1000*int64(time.Millisecond))

	// String formatted as year, month, day, hour, minute, and second
	// formattedTime := timeObj.Format("2006-01-02 15:04:05.000")

	formattedTime := fmt.Sprintf("%v", timeObj)

	return formattedTime
}

func getPriKeyBySeed(seed string) (crypto.PrivKey, error) {
	aStringToHash := []byte(seed)
	sha256Bytes := sha256.Sum256(aStringToHash)
	keySeed := int64(binary.BigEndian.Uint64(sha256Bytes[:]))
	r := rand.New(rand.NewSource(int64(keySeed)))
	// Set your own keypair
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, -1, r)
	return priv, err
}

func bytesToHexString(input []byte) string {
	hexString := "0x"
	for _, b := range input {
		hexString += fmt.Sprintf("%02x", b)
	}
	return hexString
}

// Hash 128 bit for specified string .
func hash128(bytes []byte) (hashKey string) {
	shaHash := md5.Sum(bytes)
	hashKey = hex.EncodeToString(shaHash[:])
	return hashKey
}

// Hash 256 bit for specified string .
func hash256(bytes []byte) (hashKey string) {
	shaHash := sha256.Sum256(bytes)
	hashKey = hex.EncodeToString(shaHash[:])
	return
}

// Hash 384 bit for specified string .
func hash384(bytes []byte) (hashKey string) {
	shaHash := sha512.Sum384(bytes)
	hashKey = hex.EncodeToString(shaHash[:])
	return
}

// Hash 512 bit for specified string .
func hash512(bytes []byte) (hashKey string) {
	shaHash := sha512.Sum512(bytes)
	hashKey = hex.EncodeToString(shaHash[:])
	return
}

func getTtlFromDuration(t time.Duration) uint64 {
	return uint64(t.Milliseconds())
}

func timeNow() uint64 {
	return uint64(time.Now().UnixMilli())
}

func getRecordSignData(key string, val []byte, pubkey []byte, issuetime uint64, ttl uint64) []byte {
	sigData := make([]byte, len(key)+len(val)+len(pubkey)+16)
	i := copy(sigData, []byte(key))
	i += copy(sigData[i:], val)
	i += copy(sigData[i:], pubkey)

	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, issuetime)
	i += copy(sigData[i:], b)
	binary.LittleEndian.PutUint64(b, ttl)
	copy(sigData[i:], b)

	return sigData
}
