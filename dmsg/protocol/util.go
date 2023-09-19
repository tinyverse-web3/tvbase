package protocol

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/mtv_go_utils/crypto"
	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"google.golang.org/protobuf/proto"
)

func NewBasicData(host host.Host, pubKey string, pid pb.PID) *pb.BasicData {
	ret := &pb.BasicData{
		PeerID: host.ID().String(),
		Pubkey: pubKey,
		TS:     time.Now().UnixNano(),
		ID:     uuid.New().String(),
		PID:    pid,
		Ver:    ProtocolVersion,
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

func GenProtoData(pid pb.PID, protoData []byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, pid)
	if err != nil {
		log.Logger.Errorf("GenProtocolData: error: %v", err)
		return nil, err
	}
	err = binary.Write(buf, binary.LittleEndian, protoData)
	if err != nil {
		log.Logger.Errorf("GenProtocolData: error: %v", err)
		return nil, err
	}
	return buf.Bytes(), nil
}
