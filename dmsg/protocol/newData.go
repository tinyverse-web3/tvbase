package protocol

import (
	"time"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
)

func NewBasicData(host host.Host, destPubkey string, protocolID pb.ProtocolID) (*pb.BasicData, error) {
	var signPubKey []byte
	var err error
	signPubKey, err = crypto.MarshalPublicKey(host.Peerstore().PubKey(host.ID()))

	if err != nil {
		return nil, err
	}

	ret := &pb.BasicData{
		PeerId:     host.ID().String(),
		DestPubkey: destPubkey,
		SignPubKey: signPubKey,
		Timestamp:  time.Now().Unix(),
		Id:         uuid.New().String(),
		ProtocolID: protocolID,
		Ver:        ProtocolVersion,
	}
	return ret, nil
}

func NewEmptyBasicData(host host.Host, protocolID pb.ProtocolID) *pb.BasicData {
	ret := &pb.BasicData{
		PeerId:     host.ID().String(),
		SignPubKey: nil,
		Timestamp:  time.Now().Unix(),
		Id:         "",
		ProtocolID: protocolID,
		Ver:        ProtocolVersion,
	}
	return ret
}

func NewSuccRetCode() *pb.RetCode {
	ret := &pb.RetCode{Code: 0, Result: "success"}
	return ret
}

func NewRetCode(code int32, result string) *pb.RetCode {
	ret := &pb.RetCode{Code: code, Result: result}
	return ret
}

func NewFailRetCode(result string) *pb.RetCode {
	ret := &pb.RetCode{Code: -1, Result: result}
	return ret
}
