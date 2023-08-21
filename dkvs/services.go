package dkvs

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	pb "github.com/tinyverse-web3/tvbase/dkvs/pb"
)

// maintained by DAO
const PUBSERVICE_GUN = "gun"
const PUBSERVICE_MINER = "miner"
const PUBSERVICE_DAUTH = "dauth"

const KEY_NS_GUN = "contract"

// name:publickey (eg: 0xabc)
var dkvsServiceNameMap = map[string][]string{
	PUBSERVICE_GUN:   {"0x08011220b8f61ce7b44d1ff9a8f86186eb1062180684f785d3456c4949e7792714cb1ac1"}, // zjMGsKesWSlZnayK
	PUBSERVICE_MINER: 
					{
					"0x080112206f95bc02be8daa6ad6ca1ce753b633b86c73c2edd93950b4fe5dcda29f500c2a", // thsgMCRQoWIPwfxJ
					"0x08011220ca2573a27462d653594faf292eb8ee21ac3bc6353be5234852ddb180e2e45db9", // RWQVifzRadMFMpyZ
					"0x08011220d11339fbad7d0a9201ff993d4d2dc3081c69f8be92377003d4c2baa64381b31e", // GeFTuvVKGRhlEgrq
					},
	PUBSERVICE_DAUTH: 
					{
					"0x080112205af541d9b6d1273f886b3611bb666872318926f9cc269fd6652b40ac0574f6e5", // oIBBgepoPyhdJTYB
					"0x08011220251b0c412bf1a04d98928b37b45ef34a2f75c8f76adc6e4a9697542e43efe4f4", // WliTOjtgvyZEWZNr
					}, 
}

func IsPublicServiceName(sn string) bool {
	_, ok := dkvsServiceNameMap[sn]
	return ok
}

func IsPublicServiceKey(pubkey []byte) bool {
	v := BytesToHexString(pubkey)
	for _, vector := range dkvsServiceNameMap {
		for _, val := range vector {
			if val == v {
				return true
			}
		}
	}
	return false
}

func IsPublicServiceNameKey(sn string, pubkey []byte) bool {
	v := BytesToHexString(pubkey)
	vector, ok := dkvsServiceNameMap[sn]
	if ok {
		for _, val := range vector {
			if val == v {
				return true
			}
		}
	}
	return false
}

func IsGunService(pubkey []byte) bool {
	return IsPublicServiceNameKey(PUBSERVICE_GUN, pubkey)
}

func IsGunName(key string) bool {
	if key[0] != '/' {
		Logger.Debug("invalid key")
		return false
	}

	subkeys := strings.Split(key, "/")
	l := len(subkeys)
	if l != 3 { // subkeys[0] = ""
		return false
	}

	return subkeys[1] == KEY_NS_GUN && len(subkeys[2]) <= MinDKVSRecordKeyLength
}

func GetGunKey(name string) string {
	return "/" + KEY_NS_GUN + "/" + name
}

func BytesToHexString(input []byte) string {
	hexString := "0x"
	for _, b := range input {
		hexString += fmt.Sprintf("%02x", b)
	}
	return hexString
}

func HexStringToBytes(input string) []byte {

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

	for _, vector := range dkvsServiceNameMap {
		for _, val := range vector {
			cert := SearchCertByPubkey(cv, HexStringToBytes(val))
			if cert != nil {
				return cert
			}
		}
	}

	return nil
}

func FindPublicServiceCertWithUserPubkey(cv []*pb.Cert, userPubkey []byte) *pb.Cert {

	for _, vector := range dkvsServiceNameMap {
		for _, val := range vector {
			cert := SearchCertByPubkey(cv, HexStringToBytes(val))
			if cert != nil && VerifyCert(cert) && bytes.Equal(cert.UserPubkey, userPubkey) {
				return cert
			}
		}
	}

	return nil
}

func FindPublicServiceCertByServiceName(cv []*pb.Cert, name string) *pb.Cert {

	vector, ok := dkvsServiceNameMap[name]
	if !ok {
		return nil
	}

	for _, val := range vector {
		cert := SearchCertByPubkey(cv, HexStringToBytes(val))
		if cert != nil && VerifyCert(cert) {
			return cert
		}
	}

	return nil
}

func FindGunCert(cv []*pb.Cert) *pb.Cert {
	return FindPublicServiceCertByServiceName(cv, PUBSERVICE_GUN)
}
