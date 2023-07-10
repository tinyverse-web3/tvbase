package protocol

import (
	// "github.com/libp2p/go-libp2p/core/host"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvutil/crypto"
	keyUtil "github.com/tinyverse-web3/tvutil/key"
	"google.golang.org/protobuf/proto"
)

func AuthProtocolMsg(message proto.Message, basicData *pb.BasicData) bool {
	sign := basicData.Sign
	basicData.Sign = nil
	protoData, err := proto.Marshal(message)
	if err != nil {
		dmsgLog.Logger.Errorf("AuthProtocolMsg: failed to marshal pb message %v", err)
		return false
	}
	basicData.Sign = sign
	return verifyData(protoData, basicData.SignPubKey, sign)
}

func verifyData(protoData []byte, userPubkeyHex string, sign []byte) bool {
	srcUserPubkeyData, err := keyUtil.TranslateKeyStringToProtoBuf(userPubkeyHex)
	if err != nil {
		dmsgLog.Logger.Errorf("verifyData: TranslateKeyStringToProtoBuf error: %v", err)
		return false
	}
	srcUserPubkey, err := keyUtil.ECDSAProtoBufToPublicKey(srcUserPubkeyData)
	if err != nil {
		dmsgLog.Logger.Errorf("verifyData: Public key is not ECDSA KEY")
		return false
	}
	isVerify, err := crypto.VerifyDataSignByEcdsa(srcUserPubkey, protoData, sign)
	if err != nil {
		dmsgLog.Logger.Error(err)
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
