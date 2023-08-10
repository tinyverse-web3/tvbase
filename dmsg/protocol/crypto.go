package protocol

import (
	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvutil/crypto"
	"google.golang.org/protobuf/proto"
)

func AuthProtoMsg(message proto.Message, basicData *pb.BasicData) bool {
	sig := basicData.Sig
	basicData.Sig = nil
	protoData, err := proto.Marshal(message)
	if err != nil {
		log.Logger.Errorf("AuthProtoMsg: proto.Marshal error: %v", err)
		return false
	}
	basicData.Sig = sig
	pubkey, err := crypto.PubkeyFromHex(basicData.Pubkey)
	if err != nil {
		log.Logger.Errorf("AuthProtoMsg: crypto.PubkeyFromHex error: %v", err)
		return false
	}

	ret, err := crypto.VerifyDataSignByEcdsa(pubkey, protoData, sig)
	if err != nil {
		log.Logger.Errorf("AuthProtoMsg: crypto.VerifyDataSignByEcdsa error: %v", err)
		return false
	}
	return ret
}
