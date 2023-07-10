package protocol

import (
	"time"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
)

func NewBasicData(host host.Host, signPubKey string, destPubkey string, protocolID pb.ProtocolID) (*pb.BasicData, error) {
	ret := &pb.BasicData{
		PeerId:     host.ID().String(),
		SignPubKey: signPubKey,
		DestPubkey: destPubkey,
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
		SignPubKey: "",
		DestPubkey: "",
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
