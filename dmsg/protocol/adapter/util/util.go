package util

import (
	"fmt"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/util"
)

func GetRetCode(dataList ...any) (*pb.RetCode, error) {
	retCode := util.NewSuccRetCode()
	if len(dataList) > 1 && dataList[1] != nil {
		data, ok := dataList[1].(*pb.RetCode)
		if !ok {
			return nil, fmt.Errorf("GetRetCode: fail to cast dataList[1] to *pb.RetCode")
		} else {
			if data == nil {
				fmt.Printf("GetRetCode: data == nil")
				return nil, fmt.Errorf("GetRetCode: data == nil")
			}
			retCode = data
		}
	}
	return retCode, nil
}
