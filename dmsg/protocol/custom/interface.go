package custom

import (
	"context"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
)

type Service interface {
	Request(peerId string, pid string, content []byte) (*pb.CustomProtocolReq, chan any, error)
}

type ClientHandle interface {
	GetProtocolID() string
	SetCtx(ctx context.Context)
	SetService(service Service)
}

type ServerHandle interface {
	GetProtocolID() string
	SetCtx(ctx context.Context)
	HandleRequest(req *pb.CustomProtocolReq) ([]byte, *pb.RetCode, error)
}
