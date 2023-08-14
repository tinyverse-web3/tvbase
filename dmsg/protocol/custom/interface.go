package custom

import (
	"context"

	"github.com/tinyverse-web3/tvbase/dmsg/pb"
)

type Service interface {
	Request(customProtocolId string, peerId string, content []byte) error
}

type ClientHandle interface {
	GetProtocolID() string
	SetCtx(ctx context.Context)
	SetService(service Service)
	HandleResponse(request *pb.CustomProtocolReq, res *pb.CustomProtocolRes) error
}

type ServerHandle interface {
	GetProtocolID() string
	SetCtx(ctx context.Context)
	HandleRequest(req *pb.CustomProtocolReq) error
	HandleResponse(req *pb.CustomProtocolReq, res *pb.CustomProtocolRes) error
}

type ResponseParam struct {
	PID     string
	Service ServerHandle
}
