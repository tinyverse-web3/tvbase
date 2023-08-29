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
	PUBSERVICE_GUN: {"0x080112205e1ef2628b9d27e450add495945e04eca51ef79a75925b77d2d31597680a4a8f"},
	PUBSERVICE_MINER: {
		"0x080112203da94688b4a1bf2635df6abf097cc6c5b8d9e61fe1ddf823ffd31cbcfe9e5f15",
		"0x080112205e1ef2628b9d27e450add495945e04eca51ef79a75925b77d2d31597680a4a8f", // Miner for create Default Score
		//"0x08011220ca2573a27462d653594faf292eb8ee21ac3bc6353be5234852ddb180e2e45db9", // RWQVifzRadMFMpyZ
		//"0x08011220d11339fbad7d0a9201ff993d4d2dc3081c69f8be92377003d4c2baa64381b31e", // GeFTuvVKGRhlEgrq
	},
	PUBSERVICE_DAUTH: {
		"0x08011220a7463f0275aa0826c4669ff4a4bc5182a3edb619952676b42fc7c563235ac85c",
		//"0x08011220251b0c412bf1a04d98928b37b45ef34a2f75c8f76adc6e4a9697542e43efe4f4", // WliTOjtgvyZEWZNr
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
