package protocol

import (
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	dmsgLog "github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"google.golang.org/protobuf/proto"
)

func AuthProtocolMsg(message proto.Message, data *pb.BasicData, checkPeerId bool) bool {
	sign := data.Sign
	data.Sign = nil
	bin, err := proto.Marshal(message)
	if err != nil {
		dmsgLog.Logger.Error(err)
		return false
	}
	data.Sign = sign

	peerId, err := peer.Decode(data.PeerId)
	if err != nil {
		dmsgLog.Logger.Error(err)
		return false
	}

	return VerifyData(bin, []byte(sign), peerId, data.SignPubKey, checkPeerId)
}

func VerifyData(data []byte, signature []byte, peerId peer.ID, pubKeyData []byte, checkPeerId bool) bool {
	key, err := crypto.UnmarshalPublicKey(pubKeyData)
	if err != nil {
		dmsgLog.Logger.Warnln(err, "Failed to extract key from message key data")
		return false
	}

	if checkPeerId {
		idFromKey, err := peer.IDFromPublicKey(key)
		if err != nil {
			dmsgLog.Logger.Error(err)
			return false
		}
		if idFromKey != peerId {
			dmsgLog.Logger.Error("node id and provided public key mismatch")
			return false
		}
	}

	res, err := key.Verify(data, signature)
	if err != nil {
		dmsgLog.Logger.Error(err)
		return false
	}

	return res
}

func SignProtocolMsg(message proto.Message, host host.Host) ([]byte, error) {
	data, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	signBytes, err := SignData(data, host)
	if err != nil {
		return nil, err
	}
	return signBytes, nil
}

func SignData(data []byte, host host.Host) ([]byte, error) {
	key := host.Peerstore().PrivKey(host.ID())
	res, err := key.Sign(data)
	return res, err
}
