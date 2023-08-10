package protocol

import (
	"bytes"
	"encoding/binary"

	"github.com/tinyverse-web3/tvbase/dmsg/common/log"
	"github.com/tinyverse-web3/tvbase/dmsg/pb"
)

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
