package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/tinyverse-web3/mtv_go_utils/crypto"
	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"google.golang.org/protobuf/proto"
)

func NewBasicData(host host.Host, reqPubKey string, proxyPubkey string, pid pb.PID) *pb.BasicData {
	ret := &pb.BasicData{
		PeerID:      host.ID().String(),
		Pubkey:      reqPubKey,
		ProxyPubkey: proxyPubkey,
		TS:          time.Now().Unix(),
		ID:          uuid.New().String(),
		PID:         pid,
		Ver:         DataVersion,
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
	pubkeyStr := basicData.Pubkey
	if basicData.ProxyPubkey != "" {
		pubkeyStr = basicData.ProxyPubkey
	}
	pubkey, err := crypto.PubkeyFromHex(pubkeyStr)
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

func GetBasicData(protoData any) (*pb.BasicData, error) {
	v := reflect.ValueOf(protoData)
	if v.Kind() != reflect.Ptr {
		fmt.Print("GetBasicData: protoData is not a pointer")
		return nil, fmt.Errorf("GetBasicData: protoData is not a pointer")
	}
	reflactValue := v.Elem().FieldByName("BasicData")
	if !reflactValue.IsValid() {
		fmt.Print("GetBasicData: protoData.BasicData is invalid")
		return nil, fmt.Errorf("GetBasicData: protoData.BasicData is invalid")
	}
	data := reflactValue.Interface()
	ret, ok := data.(*pb.BasicData)
	if !ok {
		fmt.Print("GetBasicData: protoData.BasicData is not a *pb.BasicData")
		return nil, fmt.Errorf("GetBasicData: protoData.BasicData is not a *pb.BasicData")
	}
	return ret, nil
}

func GetRetCode(protoData any) (*pb.RetCode, error) {
	v := reflect.ValueOf(protoData)
	if v.Kind() != reflect.Ptr {
		fmt.Print("GetRetCode: protoData is not a pointer")
		return nil, fmt.Errorf("GetRetCode: protoData is not a pointer")
	}
	reflactValue := v.Elem().FieldByName("RetCode")
	if !reflactValue.IsValid() {
		fmt.Print("GetRetCode: protoData.RetCode is invalid")
		return nil, fmt.Errorf("GetRetCode: protoData.RetCode is invalid")
	}
	data := reflactValue.Interface()
	ret, ok := data.(*pb.RetCode)
	if !ok {
		fmt.Print("GetRetCode: protoData.RetCode is not a *pb.RetCode")
		return nil, fmt.Errorf("GetRetCode: protoData.RetCode is not a *pb.RetCode")
	}
	return ret, nil
}

func SetRetCode(protoData any, code int32, result string) error {
	retCode, err := GetRetCode(protoData)
	if err != nil {
		return err
	}

	retCode.Code = code
	retCode.Result = result
	return nil
}

func SetBasicSig(protoData any, sig []byte) error {
	basicData, err := GetBasicData(protoData)
	if err != nil {
		return err
	}
	basicData.Sig = sig
	return nil
}
