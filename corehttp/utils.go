package corehttp

import (
	"net"
	"os"
	"path/filepath"

	"github.com/gogo/protobuf/proto"
	ds "github.com/ipfs/go-datastore"
	recpb "github.com/libp2p/go-libp2p-record/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-base32"
	manet "github.com/multiformats/go-multiaddr/net"
	dkvs_pb "github.com/tinyverse-web3/tvbase/dkvs/pb"
)

const (
	defaultPathName = ".tvnode"
	defaultPathRoot = "~/" + defaultPathName
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

func isPrivateNode(addInfo peer.AddrInfo) bool {
	addrs := addInfo.Addrs
	isPrivate := false
	for _, addr := range addrs {
		ip, err := manet.ToIP(addr)
		if err == nil && isPrivateIP(ip) {
			isPrivate = true
			break
		}
	}
	return isPrivate
}

func getTvBaseDataDir() string {
	relPath := defaultPathRoot
	_, err := os.Stat(relPath)
	if err != nil || os.IsNotExist(err) {
		relPath = "./"
	}
	absPath, err := filepath.Abs(relPath)
	if err != nil {
		Logger.Errorf("getTvBaseDataDir()--->Failed to get absolute path: %v", err)
		return relPath
	}
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
