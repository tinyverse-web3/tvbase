package protocol

import (
	utilCrypto "github.com/tinyverse-web3/mtv_go_utils/crypto"
	utilKey "github.com/tinyverse-web3/mtv_go_utils/key"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
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
	if basicData.ProxyPubkey != "" {
		return verifyData(protoData, basicData.ProxyPubkey, sig)
	}
	return verifyData(protoData, basicData.Pubkey, sig)
}

func verifyData(protoData []byte, pubkeyHex string, sig []byte) bool {
	pubkeyData, err := utilKey.TranslateKeyStringToProtoBuf(pubkeyHex)
	if err != nil {
		dmsgLog.Logger.Errorf("verifyData: TranslateKeyStringToProtoBuf error: %v", err)
		return false
	}
	pubkey, err := utilKey.ECDSAProtoBufToPublicKey(pubkeyData)
	if err != nil {
		dmsgLog.Logger.Errorf("verifyData: Public key is not ECDSA KEY, error: %v", err)
		return false
	}
	isVerify, err := utilCrypto.VerifyDataSignByEcdsa(pubkey, protoData, sig)
	if err != nil {
		dmsgLog.Logger.Errorf("verifyData: VerifyDataSignByEcdsa error: %v", err)
		return false
	}
	return isVerify
}
