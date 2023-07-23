package protocol

import (
	// "github.com/libp2p/go-libp2p/core/host"
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

// func GetProtoMsgSignWithHost(message proto.Message, host host.Host) ([]byte, error) {
// 	data, err := proto.Marshal(message)
// 	if err != nil {
// 		return nil, err
// 	}
// 	privateKey := host.Peerstore().PrivKey(host.ID())
// 	signBytes, err := privateKey.Sign(data)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return signBytes, nil
// }
