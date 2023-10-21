package protocol

import (
	"time"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
)

func NewBasicData(host host.Host, pubKey string, proxyPubkey string, pid pb.PID) *pb.BasicData {
	ret := &pb.BasicData{
		PeerID:      host.ID().String(),
		Pubkey:      pubKey,
		ProxyPubkey: proxyPubkey,
		TS:          time.Now().Unix(),
		ID:          uuid.New().String(),
		PID:         pid,
		Ver:         ProtocolVersion,
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
