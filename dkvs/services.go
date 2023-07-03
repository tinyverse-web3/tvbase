package dkvs

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	pb "github.com/tinyverse-web3/tvbase/dkvs/pb"
)

const MaxDKVPublicSNLength int = 8

// maintained by DAO
const KEY_NS_GUN = "gun"
const KEY_NS_TX = "tx"
const KEY_NS_WALLET = "wallet"
const KEY_NS_DAUTH = "dauth"
const KEY_NS_DMSG = "dmsg"

// name:publickey (eg: 0xabc)
var dkvsServiceNameMap = map[string]string{
	KEY_NS_GUN:    "0x08011220b8f61ce7b44d1ff9a8f86186eb1062180684f785d3456c4949e7792714cb1ac1", // zjMGsKesWSlZnayK
	KEY_NS_TX:     "0x080112206f95bc02be8daa6ad6ca1ce753b633b86c73c2edd93950b4fe5dcda29f500c2a", // thsgMCRQoWIPwfxJ
	KEY_NS_WALLET: "0x080112206f95bc02be8daa6ad6ca1ce753b633b86c73c2edd93950b4fe5dcda29f500c2a",
	KEY_NS_DAUTH:  "0x080112205af541d9b6d1273f886b3611bb666872318926f9cc269fd6652b40ac0574f6e5", // oIBBgepoPyhdJTYB
	KEY_NS_DMSG:   "0x08011220ca3f355dcfcab9e3907c205f3aee9025afeed0086af86ab0f8b4e1cb2e37915d", // yYjULoePVwOnOjmO
}

func IsPublicServiceName(sn string) bool {
	_, ok := dkvsServiceNameMap[sn]
	return ok
}

func IsPublicServiceKey(pubkey []byte) bool {
	v := bytesToHexString(pubkey)
	for _, val := range dkvsServiceNameMap {
		if val == v {
			return true
		}
	}
	return false
}

func IsPublicServiceNameKey(sn string, pubkey []byte) bool {
	v := bytesToHexString(pubkey)
	value, ok := dkvsServiceNameMap[sn]
	if ok {
		if value == v {
			return true
		}
	}
	return false
}

func IsGunService(pubkey []byte) bool {
	key, ok := dkvsServiceNameMap[KEY_NS_GUN]
	if ok {
		if key == bytesToHexString(pubkey) {
			return true
		}
	}
	return false
}

func IsGunName(key string) bool {
	if key[0] != '/' {
		Logger.Error("invalid key")
		return false
	}

	subkeys := strings.Split(key, "/")
	if len(subkeys) < 3 { // subkeys[0] = ""
		Logger.Error("invalid key")
		return false
	}

	return subkeys[1] == KEY_NS_GUN && len(subkeys[2]) <= MinDKVSRecordKeyLength
}

func GetGunKey(name string) string {
	return "/" + KEY_NS_GUN + "/" + name
}

func GetGUNPubKey() []byte {
	pubkey := dkvsServiceNameMap[KEY_NS_GUN]
	return hexStringToBytes(pubkey)
}

func bytesToHexString(input []byte) string {
	hexString := "0x"
	for _, b := range input {
		hexString += fmt.Sprintf("%02x", b)
	}
	return hexString
}

func hexStringToBytes(input string) []byte {

	var byteArray []byte
	var err error
	if len(input) > 0 && input[0:2] == "0x" {
		byteArray, err = hex.DecodeString(input[2:])
	} else {
		byteArray, err = hex.DecodeString(input)
	}

	if err != nil {
		return nil
	}

	return byteArray
}

func FindPublicServiceCert(cv []*pb.Cert) *pb.Cert {

	for _, val := range dkvsServiceNameMap {
		cert := SearchCertByPubkey(cv, hexStringToBytes(val))
		if cert != nil {
			return cert
		}
	}

	return nil
}

func FindPublicServiceCertWithUserPubkey(cv []*pb.Cert, userPubkey []byte) *pb.Cert {

	for _, val := range dkvsServiceNameMap {
		cert := SearchCertByPubkey(cv, hexStringToBytes(val))
		if cert != nil && VerifyCert(cert) && bytes.Equal(cert.UserPubkey, userPubkey) {
			return cert
		}
	}

	return nil
}

func FindPublicServiceCertByServiceName(cv []*pb.Cert, name string) *pb.Cert {

	pubkey, ok := dkvsServiceNameMap[name]
	if !ok {
		return nil
	}

	cert := SearchCertByPubkey(cv, hexStringToBytes(pubkey))
	if cert != nil && VerifyCert(cert) {
		return cert
	}

	return nil
}

func FindGunCert(cv []*pb.Cert) *pb.Cert {
	return FindPublicServiceCertByServiceName(cv, KEY_NS_GUN)
}
