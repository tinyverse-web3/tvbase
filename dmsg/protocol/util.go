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

func GetRetCode(dataList ...any) (*pb.RetCode, error) {
	retCode := NewSuccRetCode()
	if len(dataList) > 1 && dataList[1] != nil {
		data, ok := dataList[1].(*pb.RetCode)
		if !ok {
			return nil, fmt.Errorf("getRetCode: fail to cast dataList[1] to *pb.RetCode")
		} else {
			if data == nil {
				fmt.Printf("getRetCode: data == nil")
				return nil, fmt.Errorf("getRetCode: data == nil")
			}
			retCode = data
		}
	}
	return retCode, nil
}

func GetBasicData(requestProtoData any) (*pb.BasicData, error) {
	v := reflect.ValueOf(requestProtoData)
	if v.Kind() != reflect.Ptr {
		fmt.Print("GetBasicData: requestProtoData is not a pointer")
		return nil, fmt.Errorf("GetBasicData: requestProtoData is not a pointer")
	}
	reflactValue := v.Elem().FieldByName("BasicData")
	if !reflactValue.IsValid() {
		fmt.Print("GetBasicData: requestProtoData.BasicData is invalid")
		return nil, fmt.Errorf("GetBasicData: requestProtoData.BasicData is invalid")
	}
	basicDataInterface := reflactValue.Interface()
	basicData, ok := basicDataInterface.(*pb.BasicData)
	if !ok {
		fmt.Print("GetBasicData: requestProtoData.BasicData is not a *pb.BasicData")
		return nil, fmt.Errorf("GetBasicData: requestProtoData.BasicData is not a *pb.BasicData")
	}
	return basicData, nil
}
