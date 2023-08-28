package protocol

import (
	"crypto/ecdsa"

	"github.com/tinyverse-web3/tvbase/common/key"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"google.golang.org/protobuf/proto"
)

func EcdsaAuthProtocolMsg(message proto.Message, basicData *pb.BasicData) (bool, error) {
	sig := basicData.Sig
	basicData.Sig = nil
	protoData, err := proto.Marshal(message)
	if err != nil {
		dmsgLog.Logger.Errorf("EcdsaAuthProtocolMsg: failed to marshal pb message %v", err)
		return false, nil
	}
	basicData.Sig = sig
	pubkey, err := key.PubkeyFromEcdsaHex(basicData.Pubkey)
	if err != nil {
		return false, err
	}
	return key.Verify(pubkey, protoData, sig)
}

func EcdsaSignProtocolMsg(message proto.Message, privateKey *ecdsa.PrivateKey) ([]byte, error) {
	data, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	signBytes, err := key.Sign(privateKey, data)
	return signBytes, err
}