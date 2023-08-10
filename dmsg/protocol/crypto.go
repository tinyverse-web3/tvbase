package protocol

import (
	// "github.com/libp2p/go-libp2p/core/host"
	"crypto/ecdsa"

	tvbaseKey "github.com/tinyverse-web3/tvbase/common/key"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvutil/crypto"
	"github.com/tinyverse-web3/tvutil/key"
	"google.golang.org/protobuf/proto"
)

func AuthProtocolMsg(message proto.Message, basicData *pb.BasicData) bool {
	sig := basicData.Sig
	basicData.Sig = nil
	protoData, err := proto.Marshal(message)
	if err != nil {
		dmsgLog.Logger.Errorf("AuthProtocolMsg: failed to marshal pb message %v", err)
		return false
	}
	basicData.Sig = sig
	return verifyData(protoData, basicData.Pubkey, sig)
}

func verifyData(protoData []byte, pubkeyHex string, sig []byte) bool {
	pubkeyData, err := key.TranslateKeyStringToProtoBuf(pubkeyHex)
	if err != nil {
		dmsgLog.Logger.Errorf("verifyData: TranslateKeyStringToProtoBuf error: %v", err)
		return false
	}
	pubkey, err := key.ECDSAProtoBufToPublicKey(pubkeyData)
	if err != nil {
		dmsgLog.Logger.Errorf("verifyData: Public key is not ECDSA KEY, error: %v", err)
		return false
	}
	isVerify, err := crypto.VerifyDataSignByEcdsa(pubkey, protoData, sig)
	if err != nil {
		dmsgLog.Logger.Errorf("verifyData: VerifyDataSignByEcdsa error: %v", err)
		return false
	}
	return isVerify
}

func AuthProtoMsg(message proto.Message, basicData *pb.BasicData) (bool, error) {
	sig := basicData.Sig
	basicData.Sig = nil
	protoData, err := proto.Marshal(message)
	if err != nil {
		dmsgLog.Logger.Errorf("AuthProtoMsg: proto.Marshal error: %v", err)
		return false, nil
	}
	basicData.Sig = sig
	pubkey, err := crypto.PubkeyFromHex(basicData.Pubkey)
	if err != nil {
		return false, err
	}
	return tvbaseKey.Verify(pubkey, protoData, sig)
}

func EcdsaSignProtocolMsg(message proto.Message, privateKey *ecdsa.PrivateKey) ([]byte, error) {
	data, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	signBytes, err := tvbaseKey.Sign(privateKey, data)
	return signBytes, err
}
